package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/YasiruR/didcomm-prober/domain"
	"github.com/btcsuite/btcutil/base58"
	"github.com/tryfix/log"
	rand2 "math/rand"
	"strconv"
)

type Packer struct {
	enc    domain.Encryptor
	logger log.Logger
}

func NewPacker(logger log.Logger) *Packer {
	return &Packer{enc: &encryptor{}, logger: logger}
}

func (p *Packer) Pack(msg string, recPubKey, sendPubKey, sendPrvKey []byte) (domain.AuthCryptMsg, error) {
	// generating and encoding the nonce
	cekIv := []byte(strconv.Itoa(rand2.Int()))
	encodedCekIv := base64.StdEncoding.EncodeToString(cekIv)

	// generating content encryption key
	cek := make([]byte, 64)
	_, err := rand.Read(cek)
	if err != nil {
		return domain.AuthCryptMsg{}, err
	}

	// encrypting cek so it will be decrypted by recipient
	encryptedCek, err := p.enc.Box(cek, cekIv, recPubKey, sendPrvKey)
	if err != nil {
		return domain.AuthCryptMsg{}, err
	}

	// encrypting sender ver key
	encryptedSendKey, err := p.enc.SealBox(sendPubKey, recPubKey)
	if err != nil {
		return domain.AuthCryptMsg{}, err
	}

	// constructing payload
	payload := domain.Payload{
		// enc: "type",
		Typ: "JWM/1.0",
		Alg: "Authcrypt",
		Recipients: []domain.Recipient{
			{
				EncryptedKey: base64.StdEncoding.EncodeToString(encryptedCek),
				Header: domain.Header{
					Kid:    base58.Encode(recPubKey),
					Iv:     encodedCekIv,
					Sender: base64.StdEncoding.EncodeToString(encryptedSendKey),
				},
			},
		},
	}

	// base64 encoding of the payload
	data, err := json.Marshal(payload)
	if err != nil {
		return domain.AuthCryptMsg{}, err
	}
	protectedVal := base64.StdEncoding.EncodeToString(data)

	// encrypt with chachapoly1305 detached mode
	iv := []byte(strconv.Itoa(rand2.Int()))
	cipher, mac, err := p.enc.EncryptDetached(msg, iv, cek)
	if err != nil {
		return domain.AuthCryptMsg{}, err
	}

	// constructing the final message
	authCryptMsg := domain.AuthCryptMsg{
		Protected:  protectedVal,
		Iv:         base64.StdEncoding.EncodeToString(iv),
		Ciphertext: base64.StdEncoding.EncodeToString(cipher),
		Tag:        base64.StdEncoding.EncodeToString(mac),
	}

	return authCryptMsg, nil
}

func (p *Packer) Unpack(data, recPubKey, recPrvKey []byte) (text string, err error) {
	// unmarshal into authcrypt message
	var msg domain.AuthCryptMsg
	err = json.Unmarshal(data, &msg)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	// decode protected payload
	var payload domain.Payload
	decodedVal, err := base64.StdEncoding.DecodeString(msg.Protected)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	err = json.Unmarshal(decodedVal, &payload)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	if len(payload.Recipients) == 0 {
		return ``, errors.New("no recipients found")
	}
	rec := payload.Recipients[0]

	// decrypt sender verification key
	decodedSendKey, err := base64.StdEncoding.DecodeString(rec.Header.Sender) // note: array length should be checked
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	sendPubKey, err := p.enc.SealBoxOpen(decodedSendKey, recPubKey, recPrvKey)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	// decrypt cek
	decodedCek, err := base64.StdEncoding.DecodeString(rec.EncryptedKey) // note: array length should be checked
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	cekIv, err := base64.StdEncoding.DecodeString(rec.Header.Iv)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	cek, err := p.enc.BoxOpen(decodedCek, cekIv, sendPubKey, recPrvKey)
	if err != nil {
		return ``, err
	}

	// decrypt cipher text
	decodedCipher, err := base64.StdEncoding.DecodeString(msg.Ciphertext)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	mac, err := base64.StdEncoding.DecodeString(msg.Tag)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	iv, err := base64.StdEncoding.DecodeString(msg.Iv)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	textBytes, err := p.enc.DecryptDetached(decodedCipher, mac, iv, cek)
	if err != nil {
		p.logger.Error(err)
		return ``, err
	}

	return string(textBytes), nil
}
