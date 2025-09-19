package pubsub

import (
	cloudpubsub "cloud.google.com/go/pubsub"
	"context"
	"fmt"
)

// HandlerFn represents the function signature for subscription handlers
type HandlerFn func(context.Context, *cloudpubsub.Message)

// StartSubscribers starts listeners for all subscriptions in the map
func StartSubscribers(ctx context.Context, subs map[string]HandlerFn) error {
	client := GetClient()

	for subID, handler := range subs {
		sub := client.Subscription(subID)
		go func(s *cloudpubsub.Subscription, h HandlerFn, id string) {
			fmt.Printf("Listening on subscription: %s\n", id)
			err := s.Receive(ctx, func(ctx context.Context, msg *cloudpubsub.Message) {
				h(ctx, msg) // Handler decides ack/nack
			})
			if err != nil {
				fmt.Printf("Error on subscription %s: %v\n", id, err)
			}
		}(sub, handler, subID)
	}
	return nil
}
