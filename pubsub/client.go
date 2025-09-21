package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	cloudpubsub "cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

var (
	client     *cloudpubsub.Client
	clientOnce sync.Once
)

// InitClient initializes a singleton Pub/Sub client using raw JSON key
func InitClient(ctx context.Context, svcAccountJSON []byte) (*cloudpubsub.Client, error) {
	var err error
	clientOnce.Do(func() {
		logs.GetLogger().Info("Initializing Pub/Sub client")
		var creds struct {
			ProjectID string `json:"project_id"`
		}
		if err = json.Unmarshal(svcAccountJSON, &creds); err != nil {
			err = fmt.Errorf("failed to unmarshal service account json: %w", err)
			logs.GetLogger().Error("Failed to unmarshal service account JSON", zap.Error(err))
			return
		}
		logs.GetLogger().Info("Creating Pub/Sub client", zap.String("projectID", creds.ProjectID))
		client, err = cloudpubsub.NewClient(ctx, creds.ProjectID, option.WithCredentialsJSON(svcAccountJSON))
		if err != nil {
			logs.GetLogger().Error("Failed to create Pub/Sub client", zap.String("projectID", creds.ProjectID), zap.Error(err))
		} else {
			logs.GetLogger().Info("Pub/Sub client initialized successfully", zap.String("projectID", creds.ProjectID))
		}
	})
	return client, err
}

// GetClient returns already initialized client
func GetClient() *cloudpubsub.Client {
	return client
}
