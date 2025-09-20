package pubsub

import (
	cloudpubsub "cloud.google.com/go/pubsub"
	"context"
	"fmt"
)

// HandlerFn represents the function signature for subscription handlers
type HandlerFn func(context.Context, *cloudpubsub.Message)

// StartSubscribers starts listeners for all subscriptions in the map
func StartSubscribers(ctx context.Context, topics []string, handlers map[string]HandlerFn) error {
	client := GetClient()

	for _, topicName := range topics {
		handler, ok := handlers[topicName]
		if !ok {
			fmt.Printf("No handler found for topic: %s\n", topicName)
			continue
		}

		subName := fmt.Sprintf("%s-sub", topicName)
		sub := client.Subscription(subName)

		go func(s *cloudpubsub.Subscription, h HandlerFn, id string) {
			fmt.Printf("Listening on subscription: %s\n", id)
			if err := s.Receive(ctx, func(ctx context.Context, msg *cloudpubsub.Message) {
				h(ctx, msg) // handler decides ack/nack
			}); err != nil {
				fmt.Printf("Error on subscription %s: %v\n", id, err)
			}
		}(sub, handler, subName)
	}

	return nil
}

