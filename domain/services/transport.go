package services

import (
	"github.com/YasiruR/didcomm-prober/domain/models"
)

/* client-server interfaces */

type Transporter interface {
	Client
	Server
}

type Client interface {
	// Send transmits the message but marshalling should be independent of the
	// transport layer to support multiple encoding mechanisms
	Send(typ models.MsgType, data []byte, endpoint string) (res string, err error)
	Close() error
}

type Server interface {
	// Start should fail for the underlying transport failures
	Start() error
	// AddHandler creates a stream with a notifier for incoming messages.
	// Handlers with synchronous responses can be added by setting async
	// flag to false and handling reply channel in models.Message
	AddHandler(mt models.MsgType, notifier chan models.Message, async bool)
	RemoveHandler(msgType string)
	Stop() error
}
