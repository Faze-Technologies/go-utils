package secrets

import "context"

// KMSCryptor is the interface for Cloud KMS encrypt/decrypt operations.
// Use this in service constructors so callers can inject mocks in tests.
type KMSCryptor interface {
	Encrypt(ctx context.Context, keyName string, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, keyName string, ciphertext []byte) ([]byte, error)
	Close() error
}

// SecretReader is the interface for reading versioned secrets from Secret Manager.
// Use this in service constructors so callers can inject mocks in tests.
type SecretReader interface {
	GetSecret(ctx context.Context, secretVersionName string) (string, error)
	Close() error
}
