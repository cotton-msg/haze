// Package crypto предоставляет шифрование at-rest для чувствительных данных
// (push auth_secret/p256dh, refresh-токены) с использованием AES-256-GCM.
// Ключ берётся из окружения/конфига и должен быть 32 байта (hex или raw).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

var ErrInvalidKey = errors.New("crypto: encryption key must not be empty")

// Box инкапсулирует AEAD-шифр для шифрования/расшифровки значений.
type Box struct {
	aead cipher.AEAD
}

// New создаёт Box из секрета произвольной длины: ключ выводится через SHA-256.
// Пустой секрет даёт ErrInvalidKey — шифрование отключается в таком случае.
func New(secret string) (*Box, error) {
	if secret == "" {
		return nil, ErrInvalidKey
	}
	sum := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Encrypt кодирует plaintext в base64(nonce || ciphertext).
func (b *Box) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt раскодирует значение, зашифрованное Encrypt.
func (b *Box) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	plaintext, err := b.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
