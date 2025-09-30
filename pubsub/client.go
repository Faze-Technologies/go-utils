package pubsub

import (
	"context"
	"encoding/json"

	cloudpubsub "cloud.google.com/go/pubsub"
	"github.com/Faze-Technologies/go-utils/config"
	"google.golang.org/api/option"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

type PubSub struct {
	client         *cloudpubsub.Client
	closeReceivers context.CancelFunc
}

func InitPubSub() *PubSub {
	logger := logs.GetLogger()
	svcAccMap := config.GetMap("pubSub.serviceAccount")
	projectID := config.GetString("pubSub.serviceAccount.project_id")
	svcAccountJSON, err := json.Marshal(svcAccMap)
	if err != nil {
		logger.Fatal("Invalid PubSub Service Account map", zap.Error(err))
	}
	logger.Info("Initializing Pub/Sub client")
	client, err := cloudpubsub.NewClient(context.Background(), projectID, option.WithCredentialsJSON(svcAccountJSON))
	if err != nil {
		logger.Fatal("Failed to create Pub/Sub client", zap.Error(err))
	}
	logger.Info("Pub/Sub client initialized successfully")
	return &PubSub{client: client}
}

func (ps *PubSub) ClosePubSub() error {
	if ps.closeReceivers != nil {
		ps.closeReceivers()
	}
	return ps.client.Close()
}
