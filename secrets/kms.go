package secrets

import (
	"context"
	"fmt"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

// KMSClient wraps the Google Cloud KMS client.
// Authentication uses Application Default Credentials (ADC), which resolves in order:
//  1. GOOGLE_APPLICATION_CREDENTIALS env var → explicit service account key file
//  2. gcloud user credentials (local development via `gcloud auth application-default login`)
//  3. GCE/GKE metadata server → covers Workload Identity Federation automatically
type KMSClient struct {
	client *kms.KeyManagementClient
}

// NewKMSClient creates a KMS client using Application Default Credentials.
// Returns an error if the underlying GCP client cannot be initialized.
func NewKMSClient() (*KMSClient, error) {
	client, err := kms.NewKeyManagementClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create KMS client: %w", err)
	}
	return &KMSClient{client: client}, nil
}

func (k *KMSClient) Encrypt(ctx context.Context, keyName string, plaintext []byte) ([]byte, error) {
	result, err := k.client.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      keyName,
		Plaintext: plaintext,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt with key %q: %w", keyName, err)
	}
	return result.Ciphertext, nil
}

func (k *KMSClient) Decrypt(ctx context.Context, keyName string, ciphertext []byte) ([]byte, error) {
	result, err := k.client.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       keyName,
		Ciphertext: ciphertext,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt with key %q: %w", keyName, err)
	}
	return result.Plaintext, nil
}

func (k *KMSClient) Close() error {
	return k.client.Close()
}
