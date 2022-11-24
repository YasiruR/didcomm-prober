package domain

import (
	"github.com/YasiruR/didcomm-prober/domain/models"
	"github.com/YasiruR/didcomm-prober/domain/services"
	"github.com/tryfix/log"
)

type Args struct {
	Name    string
	Port    int
	Verbose bool
	PubPort int
}

type Config struct {
	Args
	Hostname         string
	InvEndpoint      string
	ExchangeEndpoint string
	PubEndpoint      string
	LogLevel         string
}

type Container struct {
	Cfg          *Config
	KeyManager   services.KeyManager
	Packer       services.Packer
	Transporter  services.Transporter
	DidAgent     services.DIDAgent
	OOB          services.OutOfBand
	Connector    services.Connector
	Pub          services.Publisher
	Sub          services.Subscriber
	Prober       services.DIDComm
	InChan       chan models.Message
	SubChan      chan models.Message
	QueryChan    chan models.Message
	ConnDoneChan chan models.Connection
	OutChan      chan string
	Log          log.Logger
}
