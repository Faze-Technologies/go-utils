package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type SecretManagerClient struct {
	client *secretmanager.Client
}

func NewSecretManagerClient() *SecretManagerClient {
	logger := logs.GetLogger()
	svcAccMap := config.GetMap("secretManager.serviceAccount")
	if svcAccMap == nil {
		logger.Fatal("KMSClient Service Account map not configured in kms.serviceAccount")
	}

	svcAccountJSON, err := json.Marshal(svcAccMap)
	if err != nil {
		logger.Fatal("Invalid SecretManagerClient Service Account map", zap.Error(err))
	}

	logger.Info("Initializing Secret Manager client")
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx, option.WithCredentialsJSON(svcAccountJSON))
	if err != nil {
		logger.Fatal("Failed to create Secret Manager client", zap.Error(err))
	}

	logger.Info("Secret Manager client initialized successfully")
	return &SecretManagerClient{client: client}
}

func (sm *SecretManagerClient) GetSecret(ctx context.Context, secretVersionName string) (string, error) {
	logger := logs.GetLogger()
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretVersionName,
	}

	result, err := sm.client.AccessSecretVersion(ctx, req)
	if err != nil {
		logger.Error("Failed to access secret version", zap.String("secret", secretVersionName), zap.Error(err))
		return "", fmt.Errorf("failed to access secret version: %w", err)
	}

	return string(result.Payload.Data), nil
}

func (sm *SecretManagerClient) Close() error {
	return sm.client.Close()
}
