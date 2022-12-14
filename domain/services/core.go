package services

import (
	"github.com/YasiruR/didcomm-prober/domain/messages"
	"github.com/YasiruR/didcomm-prober/domain/models"
)

/* core services */

type Agent interface {
	Invite() (url string, err error)
	SyncAccept(encodedInv string) error
	Accept(encodedInv string) (sender string, err error)
	SendMessage(typ, to, text string) error
	ReadMessage(msg models.Message) (text string, err error)
	Peer(label string) (models.Peer, error)
}

type DIDUtils interface {
	CreateDIDDoc(svcs []models.Service) messages.DIDDocument
	CreatePeerDID(doc messages.DIDDocument) (did string, err error)
	ValidatePeerDID(did string) error
}

type Connector interface {
	CreateConnReq(label, pthid, did string, encDidDoc messages.AuthCryptMsg) (messages.ConnReq, error)
	ParseConnReq(data []byte) (label, pthId, peerDid string, encDocBytes []byte, err error)
	CreateConnRes(pthId, did string, encDidDoc messages.AuthCryptMsg) (messages.ConnRes, error)
	ParseConnRes(data []byte) (pthId string, encDocBytes []byte, err error)
}

type OutOfBand interface {
	CreateInv(label, did string, didDoc messages.DIDDocument) (url string, err error)
	ParseInv(encInv string) (inv messages.Invitation, endpoint string, pubKey []byte, err error)
}

// Discoverer does not respond with a negative answer in any of the cases but rather it
// should only be understood as a reluctance to provide information.
// eg: The missing roles in a response does not say, "I support no roles in this protocol."
// It says, "I support the protocol but I'm providing no detail about specific roles."
// see: https://github.com/hyperledger/aries-rfcs/tree/main/features/0031-discover-features#sparse-responses
//
// Agent may use best practices to avoid fingerprinting.
// see: https://github.com/hyperledger/aries-rfcs/tree/main/features/0031-discover-features#privacy-considerations
type Discoverer interface {
	Query(endpoint, query, comment string) (fs []models.Feature, err error)
	Disclose(id, query string) messages.DiscloseFeature
}
