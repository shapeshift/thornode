package common

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"

	"crypto/md5" // nolint
)

func createHash(key string) (string, error) {
	hasher := md5.New() // nolint
	_, err := hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil)), err
}

// Decrypt the input data with passphrase
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	hash, err := createHash(passphrase)
	if err != nil {
		return nil, err
	}

	key := []byte(hash)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
