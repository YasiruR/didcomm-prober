package zmq

import (
	"encoding/json"
	"fmt"
	"github.com/YasiruR/didcomm-prober/domain"
	"github.com/YasiruR/didcomm-prober/domain/container"
	"github.com/YasiruR/didcomm-prober/domain/models"
	"github.com/YasiruR/didcomm-prober/domain/services"
	zmq "github.com/pebbe/zmq4"
	"github.com/tryfix/log"
	"sync"
)

type handler struct {
	async    bool
	notifier chan models.Message
}

type Server struct {
	endpoint string
	skt      *zmq.Socket
	handlrs  *sync.Map
	client   services.Client
	log      log.Logger
}

func NewServer(zmqCtx *zmq.Context, c *container.Container) (*Server, error) {
	skt, err := zmqCtx.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf(`constructing zmq server socket failed - %v`, err)
	}

	if err = skt.Bind(c.Cfg.InvEndpoint); err != nil {
		return nil, fmt.Errorf(`binding zmq socket to %s failed - %v`, c.Cfg.InvEndpoint, err)
	}

	return &Server{
		endpoint: c.Cfg.InvEndpoint,
		skt:      skt,
		handlrs:  &sync.Map{},
		client:   c.Client,
		log:      c.Log,
	}, nil
}

func (s *Server) AddHandler(mt models.MsgType, notifier chan models.Message, async bool) {
	s.handlrs.Store(mt, &handler{async: async, notifier: notifier})
}

func (s *Server) RemoveHandler(msgType string) {
	s.handlrs.Delete(msgType)
}

func (s *Server) Start() error {
	for {
		msg, err := s.skt.RecvMessage(0)
		if err != nil {
			if err.Error() != errTempUnavail {
				// check if needed
			}
			s.sendAck(fmt.Errorf(`receiving zmq message by receiver failed - %v`, err))
			continue
		}

		if len(msg) != 2 {
			s.sendAck(fmt.Errorf(`received an empty/invalid message with length=%d (%s)`, len(msg), msg))
			continue
		}

		var md metadata
		if err = json.Unmarshal([]byte(msg[0]), &md); err != nil {
			s.sendAck(fmt.Errorf(`unmarshalling metadata failed - %v`, err))
			continue
		}

		if models.MsgType(md.Type) == models.TypTerminate {
			s.log.Info(fmt.Sprintf(`shutting down server (%s)`, s.endpoint))
			return nil
		}

		m := models.Message{Type: models.MsgType(md.Type), Data: []byte(msg[1])}
		h, err := s.handlrByTyp(m.Type)
		if err != nil {
			s.sendAck(fmt.Errorf(`fetching handler failed - %v`, err))
			continue
		}

		if h.async {
			h.notifier <- m
			s.sendAck(nil)
			continue
		}

		m.Reply = make(chan []byte)
		h.notifier <- m
		s.sendRes(<-m.Reply)
	}
}

func (s *Server) sendAck(err error) {
	msg := successRes
	if err != nil {
		s.log.Error(err)
		msg = failedRes
	}

	if _, sendErr := s.skt.Send(msg, 0); sendErr != nil {
		s.log.Error(fmt.Sprintf(`sending zmq ack message by receiver failed - %v`, err))
	}
}

func (s *Server) sendRes(data []byte) {
	if _, err := s.skt.Send(string(data), 0); err != nil {
		s.log.Error(fmt.Sprintf(`sending zmq response message by receiver failed - %v`, err))
	}
}

func (s *Server) handlrByTyp(msgTyp models.MsgType) (*handler, error) {
	val, ok := s.handlrs.Load(msgTyp)
	if !ok {
		return nil, fmt.Errorf(`no handler found for message type %s`, msgTyp)
	}

	h, ok := val.(*handler)
	if !ok {
		return nil, fmt.Errorf(`invalid type for handler found for message type %s - should be *handler`)
	}

	return h, nil
}

func (s *Server) Stop() error {
	if _, err := s.client.Send(models.TypTerminate, []byte(domain.MsgTerminate), s.endpoint); err != nil {
		return fmt.Errorf(`sending internal terminate message failed - %v`, err)
	}
	return nil
}
