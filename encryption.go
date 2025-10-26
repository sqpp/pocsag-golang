package pocsag

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash/crc32"
	"io"
)

// EncryptionMethod represents the type of encryption to use
type EncryptionMethod int

const (
	// EncryptionNone - No encryption (default)
	EncryptionNone EncryptionMethod = iota
	// EncryptionAES256 - AES-256 encryption with Base64 encoding
	EncryptionAES256
	// EncryptionAES128 - AES-128 encryption with Base64 encoding
	EncryptionAES128
)

// EncryptionConfig holds encryption settings
type EncryptionConfig struct {
	Method EncryptionMethod
	Key    []byte
	IV     []byte // Initialization Vector (optional, will be generated if not provided)
}

// EncryptMessage encrypts a message using the specified method
func EncryptMessage(message string, config EncryptionConfig) (string, error) {
	if config.Method == EncryptionNone {
		return message, nil
	}

	// Add CRC32 checksum for integrity verification
	crc := crc32.ChecksumIEEE([]byte(message))
	messageWithCRC := fmt.Sprintf("%s\x00%08x", message, crc)

	switch config.Method {
	case EncryptionAES256:
		return encryptAES(messageWithCRC, config.Key, 32, config.IV)
	case EncryptionAES128:
		return encryptAES(messageWithCRC, config.Key, 16, config.IV)
	default:
		return "", fmt.Errorf("unsupported encryption method: %d", config.Method)
	}
}

// DecryptMessage decrypts a message using the specified method
func DecryptMessage(encryptedMessage string, config EncryptionConfig) (string, error) {
	if config.Method == EncryptionNone {
		return encryptedMessage, nil
	}

	var decrypted string
	var err error

	switch config.Method {
	case EncryptionAES256:
		decrypted, err = decryptAES(encryptedMessage, config.Key, 32, config.IV)
	case EncryptionAES128:
		decrypted, err = decryptAES(encryptedMessage, config.Key, 16, config.IV)
	default:
		return "", fmt.Errorf("unsupported encryption method: %d", config.Method)
	}

	if err != nil {
		return "", err
	}

	// Verify CRC32 checksum
	if len(decrypted) < 9 {
		return "", fmt.Errorf("decrypted message too short for CRC verification")
	}

	// Extract CRC and message
	crcPos := len(decrypted) - 9
	if decrypted[crcPos] != '\x00' {
		return "", fmt.Errorf("invalid CRC separator")
	}

	message := decrypted[:crcPos]
	crcStr := decrypted[crcPos+1:]

	// Verify CRC
	expectedCRC := crc32.ChecksumIEEE([]byte(message))
	var actualCRC uint32
	if _, err := fmt.Sscanf(crcStr, "%08x", &actualCRC); err != nil {
		return "", fmt.Errorf("invalid CRC format: %v", err)
	}

	if expectedCRC != actualCRC {
		return "", fmt.Errorf("CRC verification failed: expected %08x, got %08x", expectedCRC, actualCRC)
	}

	return message, nil
}

// encryptAES encrypts data using AES with Base64 encoding
func encryptAES(data string, key []byte, keySize int, iv []byte) (string, error) {
	// Ensure key is the correct size
	if len(key) != keySize {
		// Hash the key to get the correct size
		hash := sha256.Sum256(key)
		key = hash[:keySize]
	}

	// Generate IV if not provided
	if len(iv) == 0 {
		iv = make([]byte, aes.BlockSize)
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return "", fmt.Errorf("failed to generate IV: %v", err)
		}
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create CTR mode
	stream := cipher.NewCTR(block, iv)

	// Encrypt data
	ciphertext := make([]byte, len(data))
	stream.XORKeyStream(ciphertext, []byte(data))

	// Prepend IV to ciphertext
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)

	// Base64 encode
	return base64.StdEncoding.EncodeToString(result), nil
}

// decryptAES decrypts Base64 encoded AES data
func decryptAES(encryptedData string, key []byte, keySize int, iv []byte) (string, error) {
	// Decode Base64
	data, err := base64.StdEncoding.DecodeString(encryptedData)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %v", err)
	}

	// Ensure key is the correct size
	if len(key) != keySize {
		// Hash the key to get the correct size
		hash := sha256.Sum256(key)
		key = hash[:keySize]
	}

	// Extract IV if not provided
	if len(iv) == 0 {
		if len(data) < aes.BlockSize {
			return "", fmt.Errorf("encrypted data too short")
		}
		iv = data[:aes.BlockSize]
		data = data[aes.BlockSize:]
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %v", err)
	}

	// Create CTR mode
	stream := cipher.NewCTR(block, iv)

	// Decrypt data
	plaintext := make([]byte, len(data))
	stream.XORKeyStream(plaintext, data)

	return string(plaintext), nil
}

// GenerateRandomKey generates a random key of the specified size
func GenerateRandomKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %v", err)
	}
	return key, nil
}

// GenerateRandomIV generates a random initialization vector
func GenerateRandomIV() ([]byte, error) {
	return GenerateRandomKey(aes.BlockSize)
}

// KeyFromPassword creates a key from a password using SHA256
func KeyFromPassword(password string, size int) []byte {
	hash := sha256.Sum256([]byte(password))
	return hash[:size]
}
