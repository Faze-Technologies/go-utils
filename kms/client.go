package kms

import (
	"context"
	"encoding/json"
	"fmt"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type KmsClient struct {
	client *kms.KeyManagementClient
}

func InitKmsClient() *KmsClient {
	logger := logs.GetLogger()
	svcAccMap := config.GetMap("kms.serviceAccount")
	if svcAccMap == nil {
		logger.Fatal("KMS Service Account map not configured in kms.serviceAccount")
	}

	svcAccountJSON, err := json.Marshal(svcAccMap)
	if err != nil {
		logger.Fatal("Invalid KMS Service Account map", zap.Error(err))
	}

	logger.Info("Initializing KMS client")
	ctx := context.Background()
	client, err := kms.NewKeyManagementClient(ctx, option.WithCredentialsJSON(svcAccountJSON))
	if err != nil {
		logger.Fatal("Failed to create KMS client", zap.Error(err))
	}

	logger.Info("KMS client initialized successfully")
	return &KmsClient{client: client}
}

func (k *KmsClient) Encrypt(ctx context.Context, keyName string, plaintext []byte) ([]byte, error) {
	req := &kmspb.EncryptRequest{
		Name:      keyName,
		Plaintext: plaintext,
	}

	result, err := k.client.Encrypt(ctx, req)
	if err != nil {
		logs.GetLogger().Error("Failed to encrypt data with KMS", zap.String("keyName", keyName), zap.Error(err))
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	return result.Ciphertext, nil
}

func (k *KmsClient) Decrypt(ctx context.Context, keyName string, ciphertext []byte) ([]byte, error) {
	req := &kmspb.DecryptRequest{
		Name:       keyName,
		Ciphertext: ciphertext,
	}

	result, err := k.client.Decrypt(ctx, req)
	if err != nil {
		logs.GetLogger().Error("Failed to decrypt data with KMS", zap.String("keyName", keyName), zap.Error(err))
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return result.Plaintext, nil
}

func (k *KmsClient) Close() error {
	return k.client.Close()
}