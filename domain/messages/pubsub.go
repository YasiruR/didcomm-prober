package messages

import "github.com/YasiruR/didcomm-prober/domain/models"

type Subscribe struct {
	Id        string        `json:"@id"`
	Type      string        `json:"@type"`
	Subscribe bool          `json:"subscribe"`
	PubKey    string        `json:"pubKey"` // base58 encoding of public key
	Topic     string        `json:"topic"`
	Member    models.Member `json:"member"`
}

type Status struct {
	Id     string        `json:"@id"`
	Type   string        `json:"@type"`
	Topic  string        `json:"topic"` // might be a redundant info in general mq systems
	Member models.Member `json:"member"`
}

type ReqGroupJoin struct {
	Label        string `json:"label"`
	Topic        string `json:"topic"`
	RequesterInv string `json:"requesterInv"`
}

type ResGroupJoin struct {
	Members []models.Member `json:"members"` // includes acceptor
}
