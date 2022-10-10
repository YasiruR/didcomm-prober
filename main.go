package main

import (
	"github.com/YasiruR/didcomm-prober/crypto"
	"github.com/tryfix/log"
)

// init prober with args recipient name
// output public key

// set recipient{name, endpoint, public key}

// send message

func main() {
	logger := log.Constructor.Log(log.WithColors(true), log.WithLevel("DEBUG"), log.WithFilePath(true))
	//cfg := cli.ParseArgs()
	//
	//tr := transport.NewHTTP(cfg.Port, logger)
	//go tr.Start()
	//
	//p, err := prober.NewProber(tr, logger)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//cli.Init(cfg, p)

	crypto.Generate(logger)
}
