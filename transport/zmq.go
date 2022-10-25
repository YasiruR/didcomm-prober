package transport

import (
	"fmt"
	"github.com/YasiruR/didcomm-prober/domain"
	zmq "github.com/pebbe/zmq4"
	"github.com/tryfix/log"
)

const (
	errTempUnavail = `resource temporarily unavailable`
)

type Zmq struct {
	server *zmq.Socket
	client *zmq.Socket
	log    log.Logger
	inChan chan []byte
}

func NewZmq(c *domain.Container) (*Zmq, error) {
	ctx, err := zmq.NewContext()
	if err != nil {
		return nil, fmt.Errorf(`zmq context initialization failed - %v`, err)
	}

	reqSkt, err := ctx.NewSocket(zmq.REQ)
	if err != nil {
		return nil, fmt.Errorf(`constructing zmq client socket failed - %v`, err)
	}

	repSkt, err := ctx.NewSocket(zmq.REP)
	if err != nil {
		return nil, fmt.Errorf(`constructing zmq server socket failed - %v`, err)
	}

	if err = repSkt.Bind(c.Cfg.InvEndpoint); err != nil {
		return nil, fmt.Errorf(`binding zmq socket to %s failed - %v`, c.Cfg.InvEndpoint, err)
	}

	return &Zmq{client: reqSkt, server: repSkt, log: c.Log, inChan: c.InChan}, nil
}

func (z *Zmq) Start() {
	for {
		msg, err := z.server.RecvMessage(0)
		if err != nil {
			if err.Error() != errTempUnavail {
				z.log.Error(fmt.Sprintf(`receiving zmq message by receiver failed - %v`, err))
			}
			continue
		}

		if len(msg) == 0 {
			z.log.Error(`received an empty message`)
			continue
		}

		z.inChan <- []byte(msg[0])

		if _, err = z.server.Send(`done`, 0); err != nil {
			z.log.Error(`sending zmq message by receiver failed - %v`, err)
		}
	}
}

// Send connects to the endpoint per each message since it is more appropriate
// with DIDComm as by nature it manifests an asynchronous simplex communication.
func (z *Zmq) Send(data []byte, endpoint string) error {
	if err := z.client.Connect(endpoint); err != nil {
		return fmt.Errorf(`connecting to zmq socket (%s) failed - %v`, endpoint, err)
	}

	if _, err := z.client.Send(string(data), 0); err != nil {
		return fmt.Errorf(`sending zmq message by sender failed - %v`, err)
	}

receive:
	if _, err := z.client.RecvMessage(0); err != nil {
		if err.Error() == errTempUnavail {
			goto receive
		}
		return fmt.Errorf(`receiving zmq message by sender failed - %v`, err)
	}

	return nil
}

func (z *Zmq) Stop() error {
	z.server.Close()
	z.client.Close()
	return nil
}