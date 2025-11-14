package secretmanager

import (
	"context"
	"encoding/json"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type SecretManager struct {
	client *secretmanager.Client
}

func InitSecretManager() *SecretManager {
	logger := logs.GetLogger()
	svcAccMap := config.GetMap("secretManager.serviceAccount")
	projectID := config.GetString("secretManager.serviceAccount.project_id")
	if projectID == "" {
		logger.Fatal("SecretManager Project ID not configured in secretManager.serviceAccount.project_id")
	}

	svcAccountJSON, err := json.Marshal(svcAccMap)
	if err != nil {
		logger.Fatal("Invalid SecretManager Service Account map", zap.Error(err))
	}

	logger.Info("Initializing Secret Manager client")
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx, option.WithCredentialsJSON(svcAccountJSON))
	if err != nil {
		logger.Fatal("Failed to create Secret Manager client", zap.Error(err))
	}

	logger.Info("Secret Manager client initialized successfully")
	return &SecretManager{client: client}
}

func (sm *SecretManager) GetSecret(ctx context.Context, secretVersionName string) (string, error) {
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

func (sm *SecretManager) Close() error {
	return sm.client.Close()
}