package pubsub

import (
	"context"
	"fmt"
	"bytes"
	"cloud.google.com/go/pubsub"
)

// Publish publishes a message to a topic
func Publish(ctx context.Context, topicID string, data *bytes.Buffer, attrs map[string]string) (string, error) {
	client := GetClient()
	topic := client.Topic(topicID)
	result := topic.Publish(ctx, &pubsub.Message{
		Data:       data.Bytes(),
		Attributes: attrs,
	})
	id, err := result.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	return id, nil
}
