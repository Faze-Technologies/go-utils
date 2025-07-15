package clients

import (
	"cloud.google.com/go/pubsub"
	"context"
	"fmt"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/utils"
	"github.com/goccy/go-json"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

type PubSubClient struct {
	client *pubsub.Client
}

func NewPubSubClient(logger *zap.Logger) *PubSubClient {
	projectID := config.GetString("pubSub.project_id")
	if projectID == "" {
		logger.Fatal("GCP Project ID not configured.")
	}

	serviceAccount := config.GetMap("pubSub")
	credentialsJSON, err := json.Marshal(serviceAccount)
	if err != nil {
		logger.Fatal("Failed to marshal credentials")
	}

	client, err := pubsub.NewClient(context.TODO(), projectID, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		logger.Fatal("Error creating PubSub client", zap.Error(err))
	}

	return &PubSubClient{
		client: client,
	}
}

func (p *PubSubClient) SendMessageToQueue(ctx context.Context, topicID, message string) error {
	logger := utils.GetLogger()

	topic := p.client.Topic(topicID)
	defer topic.Stop()

	result := topic.Publish(ctx, &pubsub.Message{
		Data: []byte(message),
	})

	_, err := result.Get(ctx)
	if err != nil {
		logger.Error("Error sending message to PubSub", zap.Error(err))
		return err
	}

	return nil
}

func (p *PubSubClient) ReceiveMessage(ctx context.Context, topicID string, processFunc func(*pubsub.Message) error) error {
	logger := utils.GetLogger()

	subID := fmt.Sprintf("%s-sub", topicID)
	sub := p.client.Subscription(subID)

	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		err := processFunc(msg)
		if err != nil {
			logger.Error("Failed to process message", zap.Error(err))
			return
		}
		msg.Ack()
	})

	if err != nil {
		logger.Error("Error receiving message from PubSub", zap.Error(err))
		return err
	}

	return nil
}
