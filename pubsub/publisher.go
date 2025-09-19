package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
)

// Publish publishes a message to a topic
func Publish(ctx context.Context, topicID string, data []byte, attrs map[string]string) (string, error) {
	client := GetClient()
	topic := client.Topic(topicID)
	var msgData []byte
	if _, ok := attrs["queueName"]; ok {
		wrappedMessage := map[string]interface{}{
			"data": string(data), // JSON string that will be base64 encoded
		}
		wrappedBytes, _ := json.Marshal(wrappedMessage)
		msgData = wrappedBytes
	} else {
		msgData = data
	}
	result := topic.Publish(ctx, &pubsub.Message{
		Data:       msgData,
		Attributes: attrs,
	})
	id, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	return id, nil
}
