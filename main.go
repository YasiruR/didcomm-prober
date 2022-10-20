package main

import (
	"github.com/YasiruR/didcomm-prober/cli"
	"github.com/YasiruR/didcomm-prober/crypto"
	"github.com/YasiruR/didcomm-prober/prober"
	"github.com/YasiruR/didcomm-prober/transport"
	"github.com/tryfix/log"
)

// create peer did
// create invitation by attaching temp peer did
// follow did-exchange

func main() {
	logger := log.Constructor.Log(log.WithColors(true), log.WithLevel("DEBUG"), log.WithFilePath(true))
	cfg := cli.ParseArgs()

	recChan := make(chan string)
	enc := crypto.NewPacker(logger)
	km := crypto.KeyManager{}
	if err := km.GenerateKeys(); err != nil {
		logger.Fatal(err)
	}

	tr := transport.NewHTTP(cfg.Port, enc, &km, recChan, logger)
	go tr.Start()

	prb, err := prober.NewProber(tr, enc, &km, logger)
	if err != nil {
		log.Fatal(err)
	}

	cli.Init(cfg, prb, recChan)
}
