package domain

import (
	"github.com/tryfix/log"
)

type Config struct {
	Name             string
	Hostname         string
	Port             int
	InvEndpoint      string
	ExchangeEndpoint string
	PubEndpoint      string
	Verbose          bool
	LogLevel         string
}

type Container struct {
	Cfg     *Config
	KS      KeyService
	Packer  Packer
	Tr      Transporter
	DS      DIDService
	OOB     OOBService
	Log     log.Logger
	InChan  chan ChanMsg
	SubChan chan ChanMsg
	OutChan chan string // todo remove
}
