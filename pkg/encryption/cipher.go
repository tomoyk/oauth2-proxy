package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Cipher provides methods to encrypt and decrypt
type Cipher interface {
	Encrypt(value []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
    EncryptInto(s *string) error
	DecryptInto(s *string) error
}

type DefaultCipher struct {}

// Encrypt is a dummy method for CommonCipher.EncryptInto support
func (c *DefaultCipher) Encrypt(value []byte) ([]byte, error) { return value, nil }

// Decrypt is a dummy method for CommonCipher.DecryptInto support
func (c *DefaultCipher) Decrypt(ciphertext []byte) ([]byte, error) { return ciphertext, nil }

// EncryptInto encrypts the value and stores it back in the string pointer
func (c *DefaultCipher) EncryptInto(s *string) error {
	return into(c.Encrypt, s)
}

// DecryptInto decrypts the value and stores it back in the string pointer
func (c *DefaultCipher) DecryptInto(s *string) error {
	return into(c.Decrypt, s)
}

type Base64Cipher struct {
	DefaultCipher
	Cipher Cipher
}

// NewBase64Cipher returns a new AES Cipher for encrypting cookie values
// and wrapping them in Base64 -- Supports Legacy encryption scheme
func NewBase64Cipher(initCipher func([]byte) (Cipher, error), secret []byte) (Cipher, error) {
	c, err := initCipher(secret)
	if err != nil {
		return nil, err
	}
	return &Base64Cipher{Cipher: c}, nil
}

// Encrypt encrypts a value with AES CFB & base64 encodes it
func (c *Base64Cipher) Encrypt(value []byte) ([]byte, error) {
	encrypted, err := c.Cipher.Encrypt([]byte(value))
	if err != nil {
		return nil, err
	}

	return []byte(base64.StdEncoding.EncodeToString(encrypted)), nil
}

// Decrypt Base64 decodes a value & decrypts it with AES CFB
func (c *Base64Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	encrypted, err := base64.StdEncoding.DecodeString(string(ciphertext))
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode value %s", err)
	}

	return c.Cipher.Decrypt(encrypted)
}

type CFBCipher struct {
	DefaultCipher
	cipher.Block
}

// NewCFBCipher returns a new AES CFB Cipher
func NewCFBCipher(secret []byte) (Cipher, error) {
	c, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	return &CFBCipher{Block: c}, err
}

// Encrypt with AES CFB
func (c *CFBCipher) Encrypt(value []byte) ([]byte, error) {
	ciphertext := make([]byte, aes.BlockSize+len(value))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to create initialization vector %s", err)
	}

	stream := cipher.NewCFBEncrypter(c.Block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], value)
	return ciphertext, nil
}

// Decrypt an AES CFB ciphertext
func (c *CFBCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("encrypted value should be at least %d bytes, but is only %d bytes", aes.BlockSize, len(ciphertext))
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(c.Block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return ciphertext, nil
}

type GCMCipher struct {
	DefaultCipher
	cipher.Block
}

// NewGCMCipher returns a new AES GCM Cipher
func NewGCMCipher(secret []byte) (Cipher, error) {
	c, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	return &GCMCipher{Block: c}, err
}

// Encrypt with AES GCM on raw bytes
func (c *GCMCipher) Encrypt(value []byte) ([]byte, error) {
	gcm, err := cipher.NewGCM(c.Block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, value, nil)
	return ciphertext, nil
}

// Decrypt an AES GCM ciphertext
func (c *GCMCipher) Decrypt(ciphertext []byte) ([]byte, error) {
	gcm, err := cipher.NewGCM(c.Block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// codecFunc is a function that takes a string and encodes/decodes it
type codecFunc func([]byte) ([]byte, error)

func into(f codecFunc, s *string) error {
	// Do not encrypt/decrypt nil or empty strings
	if s == nil || *s == "" {
		return nil
	}

	d, err := f([]byte(*s))
	if err != nil {
		return err
	}
	*s = string(d)
	return nil
}
