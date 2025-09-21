package pubsub

import (
	"context"
	"fmt"

	cloudpubsub "cloud.google.com/go/pubsub"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

// HandlerFn represents the function signature for subscription handlers
type HandlerFn func(context.Context, *cloudpubsub.Message)

type PubSubConfig struct {
	NumGoroutines          int
	MaxOutstandingMessages int
	MaxOutstandingBytes    int
}

// StartSubscribers starts listeners for all subscriptions in the map
func StartSubscribers(
	ctx context.Context,
	topics []string,
	handlers map[string]HandlerFn,
	cfg PubSubConfig,
) error {
	client := GetClient()

	for _, topicName := range topics {
		handler, ok := handlers[topicName]
		if !ok {
			logs.GetLogger().Info("No handler found for topic", zap.String("topic", topicName))
			continue
		}

		subName := fmt.Sprintf("%s-sub", topicName)
		sub := client.Subscription(subName)

		sub.ReceiveSettings = cloudpubsub.ReceiveSettings{
			NumGoroutines:          cfg.NumGoroutines,
			MaxOutstandingMessages: cfg.MaxOutstandingMessages,
			MaxOutstandingBytes:    cfg.MaxOutstandingBytes,
		}

		go func(s *cloudpubsub.Subscription, h HandlerFn, id string) {
			logs.GetLogger().Info("Listening on subscription", zap.String("subscription", id))
			if err := s.Receive(ctx, func(ctx context.Context, msg *cloudpubsub.Message) {
				h(ctx, msg) // handler decides ack/nack
			}); err != nil {
				logs.GetLogger().Error("Error on subscription", zap.String("subscription", id), zap.Error(err))
			}
		}(sub, handler, subName)
	}

	return nil
}
