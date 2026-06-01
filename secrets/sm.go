package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// SecretManagerClient wraps the Google Cloud Secret Manager client.
// Authentication uses Application Default Credentials (ADC), which resolves in order:
//  1. GOOGLE_APPLICATION_CREDENTIALS env var → explicit service account key file
//  2. gcloud user credentials (local development via `gcloud auth application-default login`)
//  3. GCE/GKE metadata server → covers Workload Identity Federation automatically
type SecretManagerClient struct {
	client *secretmanager.Client
}

// NewSecretManagerClient creates a Secret Manager client using Application Default Credentials.
// Returns an error if the underlying GCP client cannot be initialized.
func NewSecretManagerClient() (*SecretManagerClient, error) {
	client, err := secretmanager.NewClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create Secret Manager client: %w", err)
	}
	return &SecretManagerClient{client: client}, nil
}

func (sm *SecretManagerClient) GetSecret(ctx context.Context, secretVersionName string) (string, error) {
	result, err := sm.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretVersionName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret %q: %w", secretVersionName, err)
	}
	return string(result.Payload.Data), nil
}

func (sm *SecretManagerClient) Close() error {
	return sm.client.Close()
}
