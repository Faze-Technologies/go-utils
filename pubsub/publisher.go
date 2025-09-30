package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

func (ps *PubSub) Publish(ctx context.Context, topicID string, payload interface{}, attrs map[string]string) (string, error) {
	logger := logs.GetLogger()

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed to marshal payload", zap.String("topicID", topicID), zap.Error(err))
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	var finalData []byte
	if _, ok := attrs["queueName"]; ok {
		outer := map[string]string{"data": string(rawBytes)}
		if finalData, err = json.Marshal(outer); err != nil {
			logger.Error("failed to marshal outer payload", zap.String("topicID", topicID), zap.Error(err))
			return "", fmt.Errorf("failed to marshal outer payload: %w", err)
		}
	} else {
		finalData = rawBytes
	}

	topic := ps.client.Topic(topicID)
	result := topic.Publish(ctx, &pubsub.Message{Data: finalData, Attributes: attrs})

	id, err := result.Get(ctx)
	if err != nil {
		logger.Error("failed to publish message", zap.String("topicID", topicID), zap.Error(err))
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	return id, nil
}
