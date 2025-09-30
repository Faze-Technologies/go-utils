package pubsub

import (
	"context"
	"fmt"
	"sync"

	cloudpubsub "cloud.google.com/go/pubsub"
	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

type HandlerFunction func(context.Context, *cloudpubsub.Message)

func (ps *PubSub) StartSubscribers(handlers map[string]HandlerFunction) {
	logger := logs.GetLogger()
	pubsubSubscribers := config.GetSlice("pubSub.subscribers")
	receiveSettings := cloudpubsub.ReceiveSettings{
		NumGoroutines:          config.GetInt("pubSub.numGoroutines"),
		MaxOutstandingMessages: config.GetInt("pubSub.maxOutstandingMessages"),
		MaxOutstandingBytes:    config.GetInt("pubSub.maxOutstandingBytes"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	ps.closeReceivers = cancel

	var wg sync.WaitGroup
	for _, topicName := range pubsubSubscribers {
		handler, ok := handlers[topicName]
		if !ok {
			logger.Info("No handler found for topic", zap.String("topic", topicName))
			continue
		}

		subName := fmt.Sprintf("%s-sub", topicName)
		sub := ps.client.Subscription(subName)
		sub.ReceiveSettings = receiveSettings

		wg.Add(1)
		go func(sub *cloudpubsub.Subscription, subName string, handler func(context.Context, *cloudpubsub.Message)) {
			defer wg.Done()
			logger.Info("Listening on subscription", zap.String("subscription", subName))
			if err := sub.Receive(ctx, handler); err != nil {
				logger.Error("Error on subscription", zap.String("subscription", subName), zap.Error(err))
			}
		}(sub, subName, handler)
	}
	wg.Wait()
}
