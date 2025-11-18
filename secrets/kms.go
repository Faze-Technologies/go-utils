package secrets

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type KMSClient struct {
	client *kms.KeyManagementClient
}

func NewKMSClient() *KMSClient {
	logger := logs.GetLogger()
	svcAccMap := config.GetMap("kms.serviceAccount")
	if svcAccMap == nil {
		logger.Fatal("KMSClient Service Account map not configured in kms.serviceAccount")
	}

	svcAccountJSON, err := json.Marshal(svcAccMap)
	if err != nil {
		logger.Fatal("Invalid KMSClient Service Account map", zap.Error(err))
	}

	logger.Info("Initializing KMSClient client")
	ctx := context.Background()
	client, err := kms.NewKeyManagementClient(ctx, option.WithCredentialsJSON(svcAccountJSON))
	if err != nil {
		logger.Fatal("Failed to create KMSClient client", zap.Error(err))
	}

	logger.Info("KMSClient client initialized successfully")
	return &KMSClient{client: client}
}

func (k *KMSClient) Encrypt(ctx context.Context, keyName string, plaintext []byte) ([]byte, error) {
	logger := logs.GetLogger()
	req := &kmspb.EncryptRequest{
		Name:      keyName,
		Plaintext: plaintext,
	}

	result, err := k.client.Encrypt(ctx, req)
	if err != nil {
		logger.Error("Failed to encrypt data with KMSClient", zap.String("keyName", keyName), zap.Error(err))
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	return result.Ciphertext, nil
}

func (k *KMSClient) Decrypt(ctx context.Context, keyName string, ciphertext []byte) ([]byte, error) {
	logger := logs.GetLogger()
	req := &kmspb.DecryptRequest{
		Name:       keyName,
		Ciphertext: ciphertext,
	}

	result, err := k.client.Decrypt(ctx, req)
	if err != nil {
		logger.Error("Failed to decrypt data with KMSClient", zap.String("keyName", keyName), zap.Error(err))
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return result.Plaintext, nil
}

func (k *KMSClient) Close() error {
	return k.client.Close()
}
