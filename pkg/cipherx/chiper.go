package cipherx

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	DEKSize   = 32
	NonceSize = 12
	Version   = "1"
)

type Cipher struct {
	MasterKey []byte
}

func NewCipher(masterKey string) *Cipher {
	return &Cipher{
		MasterKey: []byte(masterKey),
	}
}

var ErrInvalidBlob = errors.New("invalid encrypted blob format")

func (c *Cipher) randomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, b)
	return b, err
}

func (c *Cipher) aesGCMEncrypt(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce, err = c.randomBytes(gcm.NonceSize())
	if err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func (c *Cipher) aesGCMDecrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (c *Cipher) Encrypt(plainText string) (string, error) {
	if len(c.MasterKey) != 32 {
		return "", fmt.Errorf("HMAC Master Key must be 32 bytes, got %d", len(c.MasterKey))
	}

	dek, err := c.randomBytes(DEKSize)
	if err != nil {
		return "", err
	}

	nonceData, encData, err := c.aesGCMEncrypt(dek, []byte(plainText))
	if err != nil {
		return "", err
	}

	nonceDEK, encDEK, err := c.aesGCMEncrypt(c.MasterKey, dek)
	if err != nil {
		return "", err
	}

	blob := strings.Join([]string{
		Version,
		base64.StdEncoding.EncodeToString(nonceDEK),
		base64.StdEncoding.EncodeToString(encDEK),
		base64.StdEncoding.EncodeToString(nonceData),
		base64.StdEncoding.EncodeToString(encData),
	}, ":")

	return blob, nil
}

func (c *Cipher) Decrypt(blob string) (string, error) {
	parts := strings.Split(blob, ":")
	if len(parts) != 5 {
		return "", ErrInvalidBlob
	}

	nonceDEK_b64 := parts[1]
	encDEK_b64 := parts[2]
	nonceData_b64 := parts[3]
	encData_b64 := parts[4]

	nonceDEK, err := base64.StdEncoding.DecodeString(nonceDEK_b64)
	if err != nil {
		return "", err
	}
	encDEK, err := base64.StdEncoding.DecodeString(encDEK_b64)
	if err != nil {
		return "", err
	}
	nonceData, err := base64.StdEncoding.DecodeString(nonceData_b64)
	if err != nil {
		return "", err
	}
	encData, err := base64.StdEncoding.DecodeString(encData_b64)
	if err != nil {
		return "", err
	}

	dek, err := c.aesGCMDecrypt(c.MasterKey, nonceDEK, encDEK)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt DEK: %w", err)
	}

	plain, err := c.aesGCMDecrypt(dek, nonceData, encData)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %w", err)
	}

	return string(plain), nil
}
