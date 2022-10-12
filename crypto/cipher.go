package crypto

import (
	chacha "github.com/GoKillers/libsodium-go/crypto/aead/chacha20poly1305ietf"
	"github.com/GoKillers/libsodium-go/cryptobox"
)

type encryptor struct{}

func (e *encryptor) Box(payload, nonce, peerPubKey, mySecKey []byte) (encMsg []byte, err error) {
	// adding noise to the cek nonce such that it satisfies crypto box function
	for i := 0; i < 5; i++ {
		nonce = append(nonce, 48)
	}

	encMsg, _ = cryptobox.CryptoBoxEasy(payload, nonce, peerPubKey, mySecKey)
	return encMsg, nil
}

func (e *encryptor) BoxOpen(cipher, nonce, peerPubKey, mySecKey []byte) (msg []byte, err error) {
	// adding noise to the cek nonce such that it satisfies crypto box open function
	for i := 0; i < 5; i++ {
		nonce = append(nonce, 48)
	}

	msg, _ = cryptobox.CryptoBoxOpenEasy(cipher, nonce, peerPubKey, mySecKey)
	return msg, nil
}

func (e *encryptor) SealBox(payload, peerPubKey []byte) (encMsg []byte, err error) {
	encMsg, _ = cryptobox.CryptoBoxSeal(payload, peerPubKey)
	return encMsg, nil
}

func (e *encryptor) SealBoxOpen(cipher, peerPubKey, mySecKey []byte) (msg []byte, err error) {
	msg, _ = cryptobox.CryptoBoxSealOpen(cipher, peerPubKey, mySecKey)
	return msg, nil
}

func (e *encryptor) EncryptDetached(msg string, nonce, key []byte) (cipher, mac []byte, err error) {
	var convertedIv [chacha.NonceBytes]byte
	copy(convertedIv[:], nonce)

	var convertedCek [chacha.KeyBytes]byte
	copy(convertedCek[:], key)

	cipher, mac = chacha.EncryptDetached([]byte(msg), nil, &convertedIv, &convertedCek)
	return cipher, mac, nil
}

func (e *encryptor) DecryptDetached(cipher, mac, nonce, key []byte) (msg []byte, err error) {
	var convertedIv [chacha.NonceBytes]byte
	copy(convertedIv[:], nonce)

	var convertedCek [chacha.KeyBytes]byte
	copy(convertedCek[:], key)
	msg, err = chacha.DecryptDetached(cipher, mac, nil, &convertedIv, &convertedCek)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
