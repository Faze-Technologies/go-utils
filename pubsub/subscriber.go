package pubsub

import (
	"context"
	"fmt"
	"sync"

	cloudpubsub "cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

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

			wrappedHandler := func(ctx context.Context, msg *cloudpubsub.Message) {
				// Extract trace context from message attributes — this links the
				// subscriber span to the publisher's trace in SigNoz, showing the
				// full async flow: HTTP request → publish → subscriber → handler.
				propagatedCtx := otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Attributes))

				tracer := otel.Tracer("pubsub")
				spanCtx, span := tracer.Start(propagatedCtx, subName,
					trace.WithSpanKind(trace.SpanKindConsumer),
					trace.WithAttributes(
						attribute.String("messaging.system", "pubsub"),
						attribute.String("messaging.destination", subName),
						attribute.String("messaging.message_id", msg.ID),
					),
				)
				defer span.End()

				logs.WithContext(spanCtx).Info("Processing pubsub message",
					zap.String("subscription", subName),
					zap.String("messageId", msg.ID),
				)

				handler(spanCtx, msg)
			}

			if err := sub.Receive(ctx, wrappedHandler); err != nil {
				logger.Error("Error on subscription", zap.String("subscription", subName), zap.Error(err))
			}
		}(sub, subName, handler)
	}
	wg.Wait()
}
