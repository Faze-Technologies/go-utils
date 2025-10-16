package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	// "fmt"
	"io"
)

func EncryptSecret(plaintextSecret, masterKey string) (string, error) {
	key := []byte(masterKey)
	plaintext := []byte(plaintextSecret)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

func DecryptSecret(encryptedSecret, masterKey string) (string, error) {
	key := []byte(masterKey)
	ciphertext, err := hex.DecodeString(encryptedSecret)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateHMACSignature(secretKey string, message string) string {
	keyBytes, err := hex.DecodeString(secretKey)
	if err != nil {
		return ""
	}

	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func ValidateHMACSignature(providedSignature, secretKey, message string) bool {
	expectedSignature := GenerateHMACSignature(secretKey, message)

	// fmt.Println("--- Inside ValidateHMACSignature ---")
	// fmt.Printf("Provided Signature (Base64): %s\n", providedSignature)
	// fmt.Printf("Expected Signature (Base64): %s\n", expectedSignature)

	decodedProvided, err := base64.RawURLEncoding.DecodeString(providedSignature)
	if err != nil {
		// fmt.Printf("ERROR: Failed to decode provided signature: %v\n", err)
		return false
	}

	decodedExpected, err := base64.RawURLEncoding.DecodeString(expectedSignature)
	if err != nil {
		return false
	}

	return hmac.Equal(decodedProvided, decodedExpected)
}

func HashBodySHA256(body []byte) string {
	hasher := sha256.New()
	hasher.Write(body)
	return hex.EncodeToString(hasher.Sum(nil))
}