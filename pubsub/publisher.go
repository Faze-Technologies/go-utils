package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"cloud.google.com/go/pubsub"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/Faze-Technologies/go-utils/logs"
	"go.uber.org/zap"
)

// injectTraceContext injects the current OTel trace context into PubSub message
// attributes so the subscriber can extract it and continue the trace.
func injectTraceContext(ctx context.Context, attrs map[string]string) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(attrs))
}

func (ps *PubSub) Publish(ctx context.Context, topicID string, payload interface{}, attrs map[string]string) (string, error) {
	tracer := otel.Tracer("pubsub")
	spanCtx, span := tracer.Start(ctx, topicID,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "pubsub"),
			attribute.String("messaging.destination", topicID),
		),
	)
	defer span.End()

	logger := logs.WithContext(spanCtx)

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed to marshal payload", zap.String("topicID", topicID), zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	var finalData []byte
	if _, ok := attrs["queueName"]; ok {
		outer := map[string]string{"data": string(rawBytes)}
		if finalData, err = json.Marshal(outer); err != nil {
			logger.Error("failed to marshal outer payload", zap.String("topicID", topicID), zap.Error(err))
			span.SetStatus(codes.Error, err.Error())
			return "", fmt.Errorf("failed to marshal outer payload: %w", err)
		}
	} else {
		finalData = rawBytes
	}

	// Inject span context (not parent ctx) so subscriber links to this producer span
	injectTraceContext(spanCtx, attrs)

	topic := ps.client.Topic(topicID)
	result := topic.Publish(spanCtx, &pubsub.Message{Data: finalData, Attributes: attrs})

	id, err := result.Get(spanCtx)
	if err != nil {
		logger.Error("failed to publish message", zap.String("topicID", topicID), zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	span.SetAttributes(attribute.String("messaging.message_id", id))
	return id, nil
}

func (ps *PubSub) PublishV2(ctx context.Context, topicID string, payload any, message *pubsub.Message) (string, error) {
	tracer := otel.Tracer("pubsub")
	spanCtx, span := tracer.Start(ctx, topicID,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "pubsub"),
			attribute.String("messaging.destination", topicID),
		),
	)
	defer span.End()

	logger := logs.WithContext(spanCtx)

	rawBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error("failed to marshal payload", zap.String("topicID", topicID), zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	if message == nil {
		message = &pubsub.Message{}
	}
	if message.Attributes == nil {
		message.Attributes = make(map[string]string)
	}
	if _, ok := message.Attributes["queueName"]; ok {
		outer := map[string]string{"data": string(rawBytes)}
		if message.Data, err = json.Marshal(outer); err != nil {
			logger.Error("failed to marshal outer payload", zap.String("topicID", topicID), zap.Error(err))
			span.SetStatus(codes.Error, err.Error())
			return "", fmt.Errorf("failed to marshal outer payload: %w", err)
		}
	} else {
		message.Data = rawBytes
	}

	// Inject span context (not parent ctx) so subscriber links to this producer span
	injectTraceContext(spanCtx, message.Attributes)

	topic := ps.client.Topic(topicID)
	topic.EnableMessageOrdering = message.OrderingKey != ""
	result := topic.Publish(spanCtx, message)

	id, err := result.Get(spanCtx)
	if err != nil {
		logger.Error("failed to publish message", zap.String("topicID", topicID), zap.Error(err))
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("failed to publish message: %w", err)
	}
	span.SetAttributes(attribute.String("messaging.message_id", id))
	return id, nil
}
