package generic

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// IsFieldValid checks if the field exists in the given struct, including embedded fields.
// It verifies by matching field names directly or their snake_case equivalents.
func IsFieldValid(model any, field string) bool {
	t := reflect.TypeOf(model).Elem()
	return checkFields(t, field)
}

// checkFields recursively checks for the existence of a field in the struct,
// including anonymous (embedded) fields.
func checkFields(t reflect.Type, field string) bool {
	for i := 0; i < t.NumField(); i++ {
		structField := t.Field(i)
		// Check if the field matches directly
		dbName := structField.Name
		if dbName == field || ToSnakeCase(dbName) == strings.ToLower(field) {
			return true
		}
		// If the field is embedded, recursively check its fields
		if structField.Anonymous {
			if checkFields(structField.Type, field) {
				return true
			}
		}
	}
	return false
}

// ToSnakeCase converts a "PascalCase" or "camelCase" string to "snake_case".
// Example: "PascalCase" -> "pascal_case"
func ToSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, '_')
		}
		result = append(result, r)
	}
	return strings.ToLower(string(result))
}

// GenerateRandomString creates a random alphanumeric string of the given length.
// Uses the "abcdefghijklmnopqrstuvwxyz0123456789" character set.
func GenerateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random string: %v", err))
	}
	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b)
}

// EncryptAES256 encrypts plaintext using AES-256 encryption in CTR mode.
// The key must be 32 bytes long. Returns the encrypted text encoded in base64.
func EncryptAES256(key, plaintext string) (string, error) {
	// Create AES cipher block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", errors.New("key length must be 32 bytes")
	}

	// Generate initialization vector (IV)
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt plaintext using CTR mode
	ciphertext := make([]byte, len(iv)+len(plaintext))
	copy(ciphertext, iv) // Append IV to the beginning of the ciphertext
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[len(iv):], []byte(plaintext))

	// Return base64 encoded ciphertext
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES256 decrypts a base64-encoded encrypted string using AES-256 in CTR mode.
// The key must be 32 bytes long. Returns the decrypted plaintext.
func DecryptAES256(key, encrypted string) (string, error) {
	// Create AES cipher block
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", errors.New("key length must be 32 bytes")
	}

	// Decode base64 ciphertext
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted text: %w", err)
	}

	// Ensure ciphertext length is valid
	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}
	iv := ciphertext[:aes.BlockSize]        // Extract IV
	ciphertext = ciphertext[aes.BlockSize:] // Extract actual ciphertext

	// Decrypt ciphertext using CTR mode
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	// Return plaintext
	return string(plaintext), nil
}
