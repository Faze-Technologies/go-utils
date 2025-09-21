package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

// Publish publishes a message to a topic
func Publish(ctx context.Context, topicID string, payload interface{}, attrs map[string]string) (string, error) {
	client := GetClient()

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		logs.GetLogger().Error("failed to marshal payload", zap.String("topicID", topicID), zap.Error(err))
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	var finalData []byte

	if _, ok := attrs["queueName"]; ok {
		outer := map[string]string{
			"data": string(rawBytes),
		}
		finalData, err = json.Marshal(outer)
		if err != nil {
			logs.GetLogger().Error("failed to marshal outer payload", zap.String("topicID", topicID), zap.Error(err))
			return "", fmt.Errorf("failed to marshal outer payload: %w", err)
		}
	} else {
		finalData = rawBytes
	}

	topic := client.Topic(topicID)
	result := topic.Publish(ctx, &pubsub.Message{
		Data:       finalData,
		Attributes: attrs,
	})

	id, err := result.Get(ctx)
	if err != nil {
		logs.GetLogger().Error("failed to publish message", zap.String("topicID", topicID), zap.Error(err))
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	return id, nil
}
