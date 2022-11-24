package messages

type PublisherStatus struct {
	Id     string `json:"@id"`
	Type   string `json:"@type"`
	Label  string `json:"label"` // todo check if DID can be used
	Active bool   `json:"active"`
	Inv    string `json:"inv"`
	Topic  string `json:"topic"` // might be a redundant info in general mq systems
}

type SubscribeMsg struct {
	Id        string   `json:"@id"`
	Type      string   `json:"@type"`
	Subscribe bool     `json:"subscribe"`
	Peer      string   `json:"peer"`
	PubKey    string   `json:"pubKey"` // base58 encoding of public key
	Topics    []string `json:"topics"`
}
