package pubsub

import (
	"encoding/json"
	"fmt"
	"github.com/YasiruR/didcomm-prober/domain"
	"github.com/YasiruR/didcomm-prober/domain/messages"
	"github.com/YasiruR/didcomm-prober/domain/models"
	"github.com/YasiruR/didcomm-prober/domain/services"
	"github.com/btcsuite/btcutil/base58"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	zmqLib "github.com/pebbe/zmq4"
	"github.com/tryfix/log"
	"net/url"
	"strings"
	"time"
)

const (
	zmqLatencyBufMilliSec = 500
)

type compactor struct {
	zEncodr *zstd.Encoder
	zDecodr *zstd.Decoder
}

type Agent struct {
	myLabel     string
	pubEndpoint string
	invs        map[string]string // invitation per each topic
	probr       services.Agent
	client      services.Client
	km          services.KeyManager
	packer      services.Packer
	subs        *subStore
	gs          *groupStore
	auth        *auth
	log         log.Logger
	outChan     chan string
	zmq         *zmq
	*compactor

	valdtr *validator
}

func NewAgent(zmqCtx *zmqLib.Context, c *domain.Container) (*Agent, error) {
	gs := newGroupStore()
	transport, err := newZmqTransport(zmqCtx, gs, c)
	if err != nil {
		return nil, fmt.Errorf(`zmq transport init for group agent failed - %v`, err)
	}

	authn, err := authenticator(c.Cfg.Name, c.Cfg.Verbose)
	if err != nil {
		return nil, fmt.Errorf(`initializing zmq authenticator failed - %v`, err)
	}

	if err = authn.setPubAuthn(transport.pub); err != nil {
		return nil, fmt.Errorf(`setting authentication on pub socket failed - %v`, err)
	}

	if err = transport.start(c.Cfg.PubEndpoint); err != nil {
		return nil, fmt.Errorf(`starting zmq transport failed - %v`, err)
	}

	zstdEncoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return nil, fmt.Errorf(`creating zstd encoder failed - %v`, err)
	}

	zstdDecoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf(`creating zstd decoder failed - %v`, err)
	}

	a := &Agent{
		myLabel:     c.Cfg.Name,
		pubEndpoint: c.Cfg.PubEndpoint,
		invs:        make(map[string]string),
		probr:       c.Prober,
		client:      c.Client,
		km:          c.KeyManager,
		packer:      c.Packer,
		log:         c.Log,
		outChan:     c.OutChan,
		subs:        newSubStore(),
		gs:          gs,
		auth:        authn,
		zmq:         transport,
		valdtr:      newValidator(),
		compactor: &compactor{
			zEncodr: zstdEncoder,
			zDecodr: zstdDecoder,
		},
	}

	a.init(c.Server)
	return a, nil
}

// init initializes handlers for subscribe and join requests (async),
// and listeners for all incoming messages eg: join requests,
// subscriptions, state changes (active/inactive) and data messages.
func (a *Agent) init(srvr services.Server) {
	// add handler for subscribe messages
	subChan := make(chan models.Message)
	srvr.AddHandler(domain.MsgTypSubscribe, subChan, false)

	// sync handler for join-requests as requester expects
	// the group-info in return
	joinChan := make(chan models.Message)
	srvr.AddHandler(domain.MsgTypGroupJoin, joinChan, false)

	// initialize internal handlers for zmq requests
	a.listener(joinChan, a.handleJoins)
	a.listener(subChan, a.handleSubscription)

	// initialize listening on subscriptions
	a.zmq.listen(typStateSkt, a.handleState)
	a.zmq.listen(typMsgSkt, a.handleData)
}

// Create constructs a group including creator's invitation
// for the group and its models.Member
func (a *Agent) Create(topic string, publisher bool) error {
	inv, err := a.probr.Invite()
	if err != nil {
		return fmt.Errorf(`generating invitation failed - %v`, err)
	}

	m := models.Member{
		Active:      true,
		Publisher:   publisher,
		Label:       a.myLabel,
		Inv:         inv,
		PubEndpoint: a.pubEndpoint,
	}

	a.invs[topic] = inv
	a.gs.addMembr(topic, m)
	if err = a.valdtr.updateHash(topic, []models.Member{m}); err != nil {
		return fmt.Errorf(`updating checksum on group-create failed - %v`, err)
	}

	return nil
}

// todo check group state from few members to see if they are consistent and valid
// todo additionally a group token as well

// todo other validations if required - eg: check for multiple groups with same name

func (a *Agent) Join(topic, acceptor string, publisher bool) error {
	// check if already joined to the topic
	if a.gs.joined(topic) {
		return fmt.Errorf(`already connected to group %s`, topic)
	}

	inv, err := a.probr.Invite()
	if err != nil {
		return fmt.Errorf(`generating invitation failed - %v`, err)
	}

	a.invs[topic] = inv
	group, err := a.reqState(topic, acceptor, inv)
	if err != nil {
		return fmt.Errorf(`requesting group state from %s failed - %v`, acceptor, err)
	}

	// adding this node as a member
	joiner := models.Member{
		Active:      true,
		Publisher:   publisher,
		Label:       a.myLabel,
		Inv:         inv,
		PubEndpoint: a.pubEndpoint,
	}
	a.gs.addMembr(topic, joiner)

	hashes := make(map[string]string)
	for _, m := range group.Members {
		if !m.Active {
			continue
		}

		checksum, err := a.addMember(topic, publisher, m)
		if err != nil {
			a.log.Error(fmt.Sprintf(`adding %s as a member failed - %v`, m.Label, err))
			continue
		}
		hashes[m.Label] = checksum
	}

	if len(group.Members) > 1 {
		if err = a.verifyJoin(acceptor, group.Members, hashes); err != nil {
			return fmt.Errorf(`group-join verification failed - %v`, err)
		}
	}

	// publish status
	time.Sleep(zmqLatencyBufMilliSec * time.Millisecond) // buffer for zmq subscription latency
	if err = a.notifyAll(topic, true, publisher); err != nil {
		return fmt.Errorf(`publishing status active failed - %v`, err)
	}

	if err = a.valdtr.updateHash(topic, append(group.Members, joiner)); err != nil {
		return fmt.Errorf(`updating checksum on group-join failed - %v`, err)
	}

	return nil
}

func (a *Agent) addMember(topic string, publisher bool, m models.Member) (checksum string, err error) {
	if err = a.connectDIDComm(m); err != nil {
		return ``, fmt.Errorf(`connecting to %s failed - %v`, m.Label, err)
	}

	checksum, err = a.subscribeData(topic, publisher, m)
	if err != nil {
		return ``, fmt.Errorf(`subscribing to topic %s with %s failed - %v`, topic, m.Label, err)
	}

	if err = a.zmq.connect(domain.RoleSubscriber, a.myLabel, topic, m); err != nil {
		return ``, fmt.Errorf(`transport connection failed - %v`, err)
	}

	// add as a member to be shared with another in future
	a.gs.addMembr(topic, m)

	if !publisher {
		return checksum, nil
	}

	s, _, err := a.serviceInfo(m.Label)
	if err != nil {
		return ``, fmt.Errorf(`fetching service info failed for peer %s - %v`, m.Label, err)
	}
	a.subs.add(topic, m.Label, s.PubKey)
	return checksum, nil
}

// verifyJoin checks if the initial member set returned by the acceptor is consistent
// across other members thus eliminating intruders in the initial state of the joiner.
// memHashs is a map with hash values indexed by the member label.
func (a *Agent) verifyJoin(accptr string, joinSet []models.Member, memHashs map[string]string) error {
	joinedChecksm, err := a.valdtr.calculate(joinSet)
	if err != nil {
		return fmt.Errorf(`calculating checksum of initial member set failed - %v`, err)
	}
	memHashs[accptr] = joinedChecksm

	invalidMems, ok := a.valdtr.verify(memHashs)
	if !ok {
		return fmt.Errorf(`at least one inconsistent member set found (%v)`, invalidMems)
	}

	return nil
}

// reqState checks if requester has already connected with acceptor
// via didcomm and if true, sends a didcomm group-join request using
// fetched peer's information. Returns the group-join response if both
// request is successful and requester is eligible.
func (a *Agent) reqState(topic, accptr, inv string) (*messages.ResGroupJoin, error) {
	s, _, err := a.serviceInfo(accptr)
	if err != nil {
		return nil, fmt.Errorf(`fetching service info failed for peer %s - %v`, accptr, err)
	}

	byts, err := json.Marshal(messages.ReqGroupJoin{
		Id:           uuid.New().String(),
		Type:         messages.JoinRequestV1,
		Label:        a.myLabel,
		Topic:        topic,
		RequesterInv: inv,
	})
	if err != nil {
		return nil, fmt.Errorf(`marshalling group-join request failed - %v`, err)
	}

	data, err := a.pack(accptr, s.PubKey, byts)
	if err != nil {
		return nil, fmt.Errorf(`packing join-req for %s failed - %v`, accptr, err)
	}

	res, err := a.client.Send(domain.MsgTypGroupJoin, data, s.Endpoint)
	if err != nil {
		return nil, fmt.Errorf(`group-join request failed - %v`, err)
	}
	a.log.Debug(`group-join response received`, res)

	unpackedMsg, err := a.probr.ReadMessage(models.Message{Type: domain.MsgTypGroupJoin, Data: []byte(res), Reply: nil})
	if err != nil {
		return nil, fmt.Errorf(`unpacking group-join response failed - %v`, err)
	}

	var resGroup messages.ResGroupJoin
	if err = json.Unmarshal([]byte(unpackedMsg), &resGroup); err != nil {
		return nil, fmt.Errorf(`unmarshalling group-join response failed - %v`, err)
	}

	return &resGroup, nil
}

func (a *Agent) Send(topic, msg string) error {
	curntMembr := a.gs.membr(topic, a.myLabel)
	if curntMembr == nil {
		return fmt.Errorf(`member information does not exist for the current member`)
	}

	if !curntMembr.Publisher {
		return fmt.Errorf(`current member is not registered as a publisher`)
	}

	subs, err := a.subs.queryByTopic(topic)
	if err != nil {
		return fmt.Errorf(`fetching subscribers for topic %s failed - %v`, topic, err)
	}

	var published bool
	for sub, key := range subs {
		data, err := a.pack(sub, key, []byte(msg))
		if err != nil {
			return fmt.Errorf(`packing data message for %s failed - %v`, sub, err)
		}

		if err = a.zmq.publish(a.zmq.dataTopic(topic, a.myLabel, sub), data); err != nil {
			return fmt.Errorf(`zmq transport error - %v`, err)
		}

		published = true
		a.log.Trace(fmt.Sprintf(`published %s to %s of %s`, msg, topic, sub))
	}

	if published {
		a.outChan <- `Published '` + msg + `' to '` + topic + `'`
	}
	return nil
}

func (a *Agent) connectDIDComm(m models.Member) error {
	_, err := a.probr.Peer(m.Label)
	if err != nil {
		u, err := url.Parse(strings.TrimSpace(m.Inv))
		if err != nil {
			return fmt.Errorf(`parsing invitation url failed - %v`, err)
		}

		inv, ok := u.Query()[`oob`]
		if !ok {
			return fmt.Errorf(`invitation url does not contain oob query param`)
		}

		if err = a.probr.SyncAccept(inv[0]); err != nil {
			return fmt.Errorf(`accepting group-member invitation failed - %v`, err)
		}
	}

	return nil
}

// subscribeData sets subscriptions via zmq for status topic of the member.
// If the member is a publisher, it proceeds with sending a subscription
// didcomm message and subscribing to message topic via zmqLib. A checksum
// of the group maintained by the added group member is returned.
func (a *Agent) subscribeData(topic string, publisher bool, m models.Member) (checksum string, err error) {
	// get my public key corresponding to this member
	subPublcKey, err := a.km.PublicKey(m.Label)
	if err != nil {
		return ``, fmt.Errorf(`fetching public key for the connection failed - %v`, err)
	}

	// B sends agent subscribe msg to member
	sm := messages.Subscribe{
		Id:        uuid.New().String(),
		Type:      messages.SubscribeV1,
		Subscribe: true,
		PubKey:    base58.Encode(subPublcKey),
		Topic:     topic,
		Member: models.Member{
			Active:      true,
			Publisher:   publisher,
			Label:       a.myLabel,
			Inv:         a.invs[topic],
			PubEndpoint: a.pubEndpoint,
		},
		Transport: messages.Transport{
			ServrPubKey:  a.auth.servr.pub,
			ClientPubKey: a.auth.client.pub,
		},
	}

	byts, err := json.Marshal(sm)
	if err != nil {
		return ``, fmt.Errorf(`marshalling subscribe message failed - %v`, err)
	}

	s, _, err := a.serviceInfo(m.Label)
	if err != nil {
		return ``, fmt.Errorf(`fetching service info failed for peer %s - %v`, m.Label, err)
	}

	data, err := a.pack(m.Label, s.PubKey, byts)
	if err != nil {
		return ``, fmt.Errorf(`packing subscribe request for %s failed - %v`, m.Label, err)
	}

	res, err := a.client.Send(domain.MsgTypSubscribe, data, s.Endpoint)
	if err != nil {
		return ``, fmt.Errorf(`sending subscribe message failed - %v`, err)
	}

	unpackedMsg, err := a.probr.ReadMessage(models.Message{Type: domain.MsgTypSubscribe, Data: []byte(res), Reply: nil})
	if err != nil {
		return ``, fmt.Errorf(`reading subscribe didcomm response failed - %v`, err)
	}

	var resSm messages.ResSubscribe
	if err = json.Unmarshal([]byte(unpackedMsg), &resSm); err != nil {
		return ``, fmt.Errorf(`unmarshalling didcomm message into subscribe response struct failed - %v`, err)
	}

	var sktMsgs *zmqLib.Socket = nil
	if resSm.Publisher {
		sktMsgs = a.zmq.msgs
	}

	if err = a.auth.setPeerAuthn(m.Label, resSm.Transport.ServrPubKey, resSm.Transport.ClientPubKey, a.zmq.state, sktMsgs); err != nil {
		return ``, fmt.Errorf(`setting zmq transport authentication failed - %v`, err)
	}

	return resSm.Checksum, nil
}

func (a *Agent) listener(inChan chan models.Message, handlerFunc func(msg *models.Message) error) {
	go func() {
		for {
			msg := <-inChan
			if err := handlerFunc(&msg); err != nil {
				a.log.Error(fmt.Sprintf(`processing message by handler failed - %v`, err))
			}
		}
	}()
}

func (a *Agent) handleSubscription(msg *models.Message) error {
	unpackedMsg, err := a.probr.ReadMessage(*msg)
	if err != nil {
		return fmt.Errorf(`reading subscribe message failed - %v`, err)
	}

	var sm messages.Subscribe
	if err = json.Unmarshal([]byte(unpackedMsg), &sm); err != nil {
		return fmt.Errorf(`unmarshalling subscribe message failed - %v`, err)
	}

	if !sm.Subscribe {
		a.subs.delete(sm.Topic, sm.Member.Label)
		return nil
	}

	if !a.validJoiner(sm.Member.Label) {
		return fmt.Errorf(`requester (%s) is not eligible`, sm.Member.Label)
	}

	var sktMsgs *zmqLib.Socket = nil
	if sm.Member.Publisher {
		sktMsgs = a.zmq.msgs
	}

	if err = a.auth.setPeerAuthn(sm.Member.Label, sm.Transport.ServrPubKey, sm.Transport.ClientPubKey, a.zmq.state, sktMsgs); err != nil {
		return fmt.Errorf(`setting zmq transport authentication failed - %v`, err)
	}

	// send response back to subscriber along with zmq server pub-key of this node
	if err = a.sendSubscribeRes(sm.Topic, sm.Member, msg); err != nil {
		return fmt.Errorf(`sending subscribe response failed - %v`, err)
	}

	if err = a.zmq.connect(domain.RoleSubscriber, a.myLabel, sm.Topic, sm.Member); err != nil {
		return fmt.Errorf(`zmq connection failed - %v`, err)
	}

	sk := base58.Decode(sm.PubKey)
	a.subs.add(sm.Topic, sm.Member.Label, sk)
	a.log.Debug(`processed subscription request`, sm)

	return nil
}

func (a *Agent) handleJoins(msg *models.Message) error {
	body, err := a.probr.ReadMessage(*msg)
	if err != nil {
		return fmt.Errorf(`reading group-join authcrypt request failed - %v`, err)
	}

	var req messages.ReqGroupJoin
	if err = json.Unmarshal([]byte(body), &req); err != nil {
		return fmt.Errorf(`unmarshalling group-join request failed - %v`, err)
	}

	if len(a.gs.membrs(req.Topic)) == 0 {
		return fmt.Errorf(`acceptor is not a member of the requested group (%s)`, req.Topic)
	}

	if !a.validJoiner(req.Label) {
		return fmt.Errorf(`group-join request denied to member (%s)`, req.Label)
	}

	byts, err := json.Marshal(messages.ResGroupJoin{
		Id:      uuid.New().String(),
		Type:    messages.JoinResponseV1,
		Members: a.gs.membrs(req.Topic),
	})
	if err != nil {
		return fmt.Errorf(`marshalling group-join response failed - %v`, err)
	}

	packedMsg, err := a.pack(req.Label, nil, byts)
	if err != nil {
		return fmt.Errorf(`packing group-join response failed - %v`, err)
	}

	// no response is sent if process failed
	msg.Reply <- packedMsg
	a.log.Debug(fmt.Sprintf(`shared group state upon join request by %s`, req.Label), string(byts))

	return nil
}

func (a *Agent) handleState(msg string) error {
	frames := strings.SplitN(msg, ` `, 2)
	if len(frames) != 2 {
		return fmt.Errorf(`received an invalid status message (length=%v)`, len(frames))
	}

	sm, err := a.extractStatus(frames[1])
	if err != nil {
		return fmt.Errorf(`extracting status message failed - %v`, err)
	}

	var validMsg string
	for exchId, encMsg := range sm.AuthMsgs {
		ok, _ := a.probr.ValidConn(exchId)
		if ok {
			validMsg = encMsg
			break
		}
	}

	if validMsg == `` {
		return fmt.Errorf(`status update is not intended to this member`)
	}

	defer func(topic string) {
		if err = a.valdtr.updateHash(topic, a.gs.membrs(topic)); err != nil {
			a.log.Error(fmt.Sprintf(`updating checksum for the group failed - %v`, err))
		}
	}(sm.Topic)

	strAuthMsg, err := a.probr.ReadMessage(models.Message{Type: domain.MsgTypGroupStatus, Data: []byte(validMsg)})
	if err != nil {
		return fmt.Errorf(`reading status didcomm message failed - %v`, err)
	}

	var member models.Member
	if err = json.Unmarshal([]byte(strAuthMsg), &member); err != nil {
		return fmt.Errorf(`unmarshalling member message failed - %v`, err)
	}

	if !member.Active {
		if member.Publisher {
			if err = a.zmq.unsubscribeData(a.myLabel, sm.Topic, member.Label); err != nil {
				return fmt.Errorf(`unsubscribing data topic failed - %v`, err)
			}
		}

		a.subs.delete(sm.Topic, member.Label)
		a.gs.deleteMembr(sm.Topic, member.Label)
		if err = a.auth.remvKeys(member.Label); err != nil {
			return fmt.Errorf(`removing zmq transport keys failed - %v`, err)
		}
		a.outChan <- member.Label + ` left group ` + sm.Topic
		return nil
	}

	a.gs.addMembr(sm.Topic, member)
	a.log.Debug(fmt.Sprintf(`group state updated for member %s in topic %s`, member.Label, sm.Topic))
	return nil
}

func (a *Agent) handleData(msg string) error {
	frames := strings.Split(msg, ` `)
	if len(frames) != 2 {
		return fmt.Errorf(`received an invalid subscribed message (%v)`, frames)
	}

	_, err := a.probr.ReadMessage(models.Message{Type: domain.MsgTypData, Data: []byte(frames[1])})
	if err != nil {
		return fmt.Errorf(`reading subscribed message failed - %v`, err)
	}

	return nil
}

// notifyAll constructs a single status message with different didcomm
// messages packed per each member of the group and includes in a map with
// exchange ID as the key.
func (a *Agent) notifyAll(topic string, active, publisher bool) error {
	comprsd, err := a.compressStatus(topic, active, publisher)
	if err != nil {
		return fmt.Errorf(`compress status failed - %v`, err)
	}

	if err = a.zmq.publish(a.zmq.stateTopic(topic), comprsd); err != nil {
		return fmt.Errorf(`zmq transport error for status - %v`, err)
	}

	a.log.Debug(fmt.Sprintf(`published status (topic: %s, active: %t, publisher: %t)`, topic, active, publisher))
	return nil
}

func (a *Agent) sendSubscribeRes(topic string, m models.Member, msg *models.Message) error {
	// to fetch if current node is a publisher of the topic
	curntMembr := a.gs.membr(topic, a.myLabel)
	if curntMembr == nil {
		return fmt.Errorf(`current member or topic does not exist in group store`)
	}

	checksum, err := a.valdtr.hash(topic)
	if err != nil {
		return fmt.Errorf(`fetching group checksum failed - %v`, err)
	}

	resByts, err := json.Marshal(messages.ResSubscribe{
		Transport: messages.Transport{
			ServrPubKey:  a.auth.servr.pub,
			ClientPubKey: a.auth.client.pub,
		},
		Publisher: curntMembr.Publisher,
		Checksum:  checksum,
	})
	if err != nil {
		return fmt.Errorf(`marshalling subscribe response failed - %v`, err)
	}

	packedMsg, err := a.pack(m.Label, nil, resByts)
	if err != nil {
		return fmt.Errorf(`packing subscribe response failed - %v`, err)
	}

	msg.Reply <- packedMsg
	return nil
}

func (a *Agent) compressStatus(topic string, active, publisher bool) ([]byte, error) {
	sm := messages.Status{Id: uuid.New().String(), Type: messages.MemberStatusV1, Topic: topic, AuthMsgs: map[string]string{}}
	byts, err := json.Marshal(models.Member{
		Label:       a.myLabel,
		Active:      active,
		Inv:         a.invs[topic],
		Publisher:   publisher,
		PubEndpoint: a.pubEndpoint,
	})
	if err != nil {
		return nil, fmt.Errorf(`marshalling member failed - %v`, err)
	}

	mems := a.gs.membrs(topic)
	for _, m := range mems {
		if m.Label == a.myLabel {
			continue
		}

		s, pr, err := a.serviceInfo(m.Label)
		if err != nil {
			return nil, fmt.Errorf(`fetching service info failed for peer %s - %v`, m.Label, err)
		}

		data, err := a.pack(m.Label, s.PubKey, byts)
		if err != nil {
			return nil, fmt.Errorf(`packing message for %s failed - %v`, m.Label, err)
		}

		sm.AuthMsgs[pr.ExchangeThId] = string(data)
	}

	encodedStatus, err := json.Marshal(sm)
	if err != nil {
		return nil, fmt.Errorf(`marshalling status message failed - %v`, err)
	}

	cmprsd := a.zEncodr.EncodeAll(encodedStatus, make([]byte, 0, len(encodedStatus)))
	a.log.Trace(fmt.Sprintf(`compressed status message (from %d to %d #bytes)`, len(encodedStatus), len(cmprsd)))
	return cmprsd, nil
}

func (a *Agent) serviceInfo(peer string) (*models.Service, *models.Peer, error) {
	pr, err := a.probr.Peer(peer)
	if err != nil {
		return nil, nil, fmt.Errorf(`no peer found - %v`, err)
	}

	var srvc *models.Service
	for _, s := range pr.Services {
		if s.Type == domain.ServcGroupJoin {
			srvc = &s
			break
		}
	}

	if srvc.Type == `` {
		return nil, nil, fmt.Errorf(`requested service (%s) is not by the peer`, domain.ServcGroupJoin)
	}

	return srvc, &pr, nil
}

func (a *Agent) extractStatus(msg string) (*messages.Status, error) {
	out, err := a.zDecodr.DecodeAll([]byte(msg), nil)
	if err != nil {
		return nil, fmt.Errorf(`decode error - %v`, err)
	}

	var sm messages.Status
	if err = json.Unmarshal(out, &sm); err != nil {
		return nil, fmt.Errorf(`unmarshal error - %v`, err)
	}

	return &sm, nil
}

// pack constructs and encodes an authcrypt message to the given receiver
func (a *Agent) pack(receiver string, recPubKey []byte, msg []byte) ([]byte, error) {
	if recPubKey == nil {
		s, _, err := a.serviceInfo(receiver)
		if err != nil {
			return nil, fmt.Errorf(`fetching service info failed for peer %s - %v`, receiver, err)
		}
		recPubKey = s.PubKey
	}

	ownPubKey, err := a.km.PublicKey(receiver)
	if err != nil {
		return nil, fmt.Errorf(`getting public key for connection with %s failed - %v`, receiver, err)
	}

	ownPrvKey, err := a.km.PrivateKey(receiver)
	if err != nil {
		return nil, fmt.Errorf(`getting private key for connection with %s failed - %v`, receiver, err)
	}

	encryptdMsg, err := a.packer.Pack(msg, recPubKey, ownPubKey, ownPrvKey)
	if err != nil {
		return nil, fmt.Errorf(`packing error - %v`, err)
	}

	data, err := json.Marshal(encryptdMsg)
	if err != nil {
		return nil, fmt.Errorf(`marshalling packed message failed - %v`, err)
	}

	return data, nil
}

func (a *Agent) parseMembrStatus(msg string) (*messages.Status, error) {
	frames := strings.Split(msg, " ")
	if len(frames) != 2 {
		return nil, fmt.Errorf(`received a message (%v) with an invalid format - frame count should be 2`, msg)
	}

	var ms messages.Status
	if err := json.Unmarshal([]byte(frames[1]), &ms); err != nil {
		return nil, fmt.Errorf(`unmarshalling publisher status failed (msg: %s) - %v`, frames[1], err)
	}

	return &ms, nil
}

// dummy validation for PoC
func (a *Agent) validJoiner(label string) bool {
	return true
}

func (a *Agent) Leave(topic string) error {
	if err := a.zmq.unsubscribeAll(a.myLabel, topic); err != nil {
		return fmt.Errorf(`zmw unsubsription failed - %v`, err)
	}

	membrs := a.gs.membrs(topic)
	if len(membrs) == 0 {
		return fmt.Errorf(`no members found`)
	}

	var publisher bool
	for _, m := range membrs {
		if m.Label == a.myLabel {
			publisher = m.Publisher
		}
	}

	if err := a.notifyAll(topic, false, publisher); err != nil {
		return fmt.Errorf(`publishing inactive status failed - %v`, err)
	}

	a.subs.deleteTopic(topic)
	a.gs.deleteTopic(topic)
	a.outChan <- `Left group ` + topic
	return nil
}

func (a *Agent) Info(topic string) (mems []models.Member) {
	// removing invitation for more clarity
	for _, m := range a.gs.membrs(topic) {
		m.Inv = ``
		mems = append(mems, m)
	}

	return mems
}

func (a *Agent) Close() error {
	// close all sockets
	return nil
}
