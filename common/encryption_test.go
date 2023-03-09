package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	. "gopkg.in/check.v1"
)

type EncryptionSuite struct{}

var _ = Suite(&EncryptionSuite{})

// NOTE: only here to keep tests functional, no longer available outside of package.
func encrypt(data []byte, passphrase string) ([]byte, error) {
	hash, err := createHash(passphrase)
	if err != nil {
		return nil, err
	}

	block, _ := aes.NewCipher([]byte(hash))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (s *EncryptionSuite) TestEncryption(c *C) {
	body := []byte("hello world!")
	passphrase := "my super secret password!"

	encryp, err := encrypt(body, passphrase)
	c.Assert(err, IsNil)

	decryp, err := Decrypt(encryp, passphrase)
	c.Assert(err, IsNil)

	c.Check(body, DeepEquals, decryp)
}

func BenchmarkEncrypt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := fmt.Sprintf("hello world! %d", i)
		passphrase := fmt.Sprintf("my super secret password! %d", i)
		result, err := encrypt([]byte(body), passphrase)
		if err != nil {
			fmt.Println(err)
			b.FailNow()
		}
		decryptResult, err := Decrypt(result, passphrase)
		if err != nil {
			fmt.Println(err)
			b.FailNow()
		}
		_ = decryptResult
	}
}
