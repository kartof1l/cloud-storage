// internal/crypto/encryption.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"
)

type EncryptionService struct {
	key []byte
}

func NewEncryptionService(secret string) *EncryptionService {
	hash := sha256.Sum256([]byte(secret))
	return &EncryptionService{key: hash[:]}
}

// EncryptFile шифрует файл перед сохранением
func (e *EncryptionService) EncryptFile(srcPath, dstPath string) error {
	input, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	ciphertext := gcm.Seal(nonce, nonce, input, nil)

	return os.WriteFile(dstPath, ciphertext, 0644)
}

// DecryptFile расшифровывает файл
func (e *EncryptionService) DecryptFile(srcPath, dstPath string) error {
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return err
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	return os.WriteFile(dstPath, plaintext, 0644)
}
