package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"golang.org/x/crypto/scrypt"
	"io"
)

const (
	// parameters here are default parameters copied from go-ethereum
	StandardScryptN = 1 << 18
	scryptR         = 8
	scryptDKLen     = 32
	StandardScryptP = 1
)

// Encrypt the input data with passphrase
func Encrypt(data []byte, passphrase string) ([]byte, error) {

	if len(data) == 0 || len(passphrase) == 0 {
		return nil, errors.New("data or passphrase should not be empty")
	}
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	derivedKey, err := scrypt.Key([]byte(passphrase), salt, StandardScryptN, scryptR, StandardScryptP, scryptDKLen)
	if err != nil {
		return nil, err
	}

	block, _ := aes.NewCipher(derivedKey)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	ciphertext = append(salt, ciphertext...)
	return ciphertext, nil
}

// Decrypt the input data with passphrase
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) < 32 || len(passphrase) == 0 {
		return nil, errors.New("incorrect data format")
	}
	salt, encData := data[:32], data[32:]
	derivedKey, err := scrypt.Key([]byte(passphrase), salt, StandardScryptN, scryptR, StandardScryptP, scryptDKLen)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := encData[:nonceSize], encData[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
