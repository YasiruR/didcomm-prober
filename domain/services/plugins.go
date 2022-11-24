package services

import "github.com/YasiruR/didcomm-prober/domain/messages"

/* dependencies */

type Transporter interface {
	// Start should fail for the underlying transport failures
	Start()
	// Send transmits the message but marshalling should be independent of the
	// transport layer to support multiple encoding mechanisms
	Send(typ string, data []byte, endpoint string) (msg []string, err error)
	Stop() error
}

type Server interface {
	Start()
	AddHandler()
	Stop() error
}

type Client interface {
	Init()
	Send(typ string, data []byte, endpoint string) (msg []string, err error)
}

type Packer interface {
	Pack(input []byte, recPubKey, sendPubKey, sendPrvKey []byte) (messages.AuthCryptMsg, error)
	Unpack(data, recPubKey, recPrvKey []byte) (output []byte, err error)
}

type Encryptor interface {
	Box(payload, nonce, peerPubKey, mySecKey []byte) (encMsg []byte, err error)
	BoxOpen(cipher, nonce, peerPubKey, mySecKey []byte) (msg []byte, err error)
	SealBox(payload, peerPubKey []byte) (encMsg []byte, err error)
	SealBoxOpen(cipher, peerPubKey, mySecKey []byte) (msg []byte, err error)
	EncryptDetached(msg, protectedVal string, nonce, key []byte) (cipher, mac []byte, err error)
	DecryptDetached(cipher, mac, protectedVal, nonce, key []byte) (msg []byte, err error)
}

type KeyManager interface {
	GenerateKeys(peer string) error
	Peer(pubKey []byte) (name string, err error)
	PublicKey(peer string) ([]byte, error)
	PrivateKey(peer string) ([]byte, error)
	GenerateInvKeys() error
	InvPublicKey() []byte
	InvPrivateKey() []byte
}
