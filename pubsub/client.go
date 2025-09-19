package pubsub

import (
	cloudpubsub "cloud.google.com/go/pubsub"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/option"
	"sync"
)

var (
	client     *cloudpubsub.Client
	clientOnce sync.Once
)

// InitClient initializes a singleton Pub/Sub client using raw JSON key
func InitClient(ctx context.Context, svcAccountJSON []byte) (*cloudpubsub.Client, error) {
	var err error
	clientOnce.Do(func() {
		var creds struct {
			ProjectID string `json:"project_id"`
		}
		if err = json.Unmarshal(svcAccountJSON, &creds); err != nil {
			err = fmt.Errorf("failed to unmarshal service account json: %w", err)
			return
		}
		client, err = cloudpubsub.NewClient(ctx, creds.ProjectID, option.WithCredentialsJSON(svcAccountJSON))
	})
	return client, err
}

// GetClient returns already initialized client
func GetClient() *cloudpubsub.Client {
	return client
}
