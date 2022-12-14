package crypto

import (
	"crypto/rand"
	"fmt"
	"golang.org/x/crypto/nacl/box"
)

type keys struct {
	pub *[32]byte
	prv *[32]byte
}

type KeyManager struct {
	invPubKey *[32]byte
	invPrvKey *[32]byte

	// can use sync.Map
	store   map[string]keys // key: peer label
	invKeys map[string]keys // key: topic
}

func NewKeyManager() *KeyManager {
	return &KeyManager{store: make(map[string]keys)}
}

func (k *KeyManager) GenerateKeys(peer string) error {
	reader := rand.Reader
	pubKey, prvKey, err := box.GenerateKey(reader)
	if err != nil {
		return err
	}

	k.store[peer] = keys{pub: pubKey, prv: prvKey}
	return nil
}

func (k *KeyManager) PrivateKey(peer string) ([]byte, error) {
	key, ok := k.store[peer]
	if !ok {
		return nil, fmt.Errorf(`no private key found for the connection with %s`, peer)
	}

	tmpPrvKey := *key.prv
	return tmpPrvKey[:], nil
}

func (k *KeyManager) PublicKey(peer string) ([]byte, error) {
	key, ok := k.store[peer]
	if !ok {
		return nil, fmt.Errorf(`no public key found for the connection with %s`, peer)
	}

	tmpPubKey := *key.pub
	return tmpPubKey[:], nil
}

func (k *KeyManager) Peer(pubKey []byte) (name string, err error) {
	for n, _ := range k.store {
		// omitting error
		storePubKey, _ := k.PublicKey(n)
		if string(storePubKey) == string(pubKey) {
			return n, nil
		}
	}

	return ``, fmt.Errorf(`could not find the requested public key (%s)`, string(pubKey))
}

func (k *KeyManager) GenerateInvKeys() error {
	// uses one key-pair for all invitations but can use separate ones for higher security
	if k.invPrvKey != nil && k.invPubKey != nil {
		return nil
	}

	reader := rand.Reader
	pubKey, prvKey, err := box.GenerateKey(reader)
	if err != nil {
		return err
	}

	k.invPrvKey = prvKey
	k.invPubKey = pubKey

	return nil
}

func (k *KeyManager) InvPrivateKey() []byte {
	tmpPrvKey := *k.invPrvKey
	return tmpPrvKey[:]
}

func (k *KeyManager) InvPublicKey() []byte {
	tmpPubKey := *k.invPubKey
	return tmpPubKey[:]
}
