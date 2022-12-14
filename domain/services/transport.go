package services

import "github.com/YasiruR/didcomm-prober/domain/models"

/* client-server interfaces */

type Transporter interface {
	Client
	Server
}

type Client interface {
	// Send transmits the message but marshalling should be independent of the
	// transport layer to support multiple encoding mechanisms
	Send(typ string, data []byte, endpoint string) (res string, err error) // todo remove res
}

type Server interface {
	// Start should fail for the underlying transport failures
	Start() error
	// AddHandler creates a stream with a notifier for incoming messages.
	// Handlers with synchronous responses can be added by setting async
	// flag to false and handling reply channel in models.Message
	AddHandler(msgType string, notifier chan models.Message, async bool)
	RemoveHandler(msgType string)
	Stop() error
}

/* message queue functions */

type GroupAgent interface {
	Create(topic string, publisher bool) error
	Join(topic, acceptor string, publisher bool) error
	Publish(topic, msg string) error
	Leave(topic string) error
}
