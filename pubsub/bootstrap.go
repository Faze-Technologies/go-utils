package pubsub

import (
	"context"
	"errors"
	"fmt"
	"time"

	cloudpubsub "cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Faze-Technologies/go-utils/config"
	"github.com/Faze-Technologies/go-utils/logs"
)

const (
	defaultAckDeadline       = 60 * time.Second
	defaultRetentionDuration = 7 * 24 * time.Hour
)

func (ps *PubSub) EnsureTopic(ctx context.Context, topicID string) (*cloudpubsub.Topic, error) {
	logger := logs.GetLogger()
	start := time.Now()
	topic := ps.client.Topic(topicID)

	exists, err := topic.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check topic exists %q: %w", topicID, err)
	}
	if exists {
		logger.Info("Pub/Sub topic already exists, skipping create",
			zap.String("topic", topicID),
			zap.Duration("duration", time.Since(start)),
		)
		return topic, nil
	}

	created, err := ps.client.CreateTopic(ctx, topicID)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			logger.Info("Pub/Sub topic already exists (created concurrently)",
				zap.String("topic", topicID),
				zap.Duration("duration", time.Since(start)),
			)
			return topic, nil
		}
		return nil, fmt.Errorf("create topic %q: %w", topicID, err)
	}
	logger.Info("Created Pub/Sub topic",
		zap.String("topic", topicID),
		zap.Duration("duration", time.Since(start)),
	)
	return created, nil
}

func (ps *PubSub) EnsureSubscription(
	ctx context.Context,
	subID string,
	topic *cloudpubsub.Topic,
	cfg cloudpubsub.SubscriptionConfig,
) (*cloudpubsub.Subscription, error) {
	logger := logs.GetLogger()
	start := time.Now()
	sub := ps.client.Subscription(subID)

	exists, err := sub.Exists(ctx)
	if err != nil {
		return nil, fmt.Errorf("check subscription exists %q: %w", subID, err)
	}
	if exists {
		logger.Info("Pub/Sub subscription already exists, skipping create",
			zap.String("subscription", subID),
			zap.String("topic", topic.ID()),
			zap.Duration("duration", time.Since(start)),
		)
		return sub, nil
	}

	cfg.Topic = topic
	if cfg.AckDeadline == 0 {
		cfg.AckDeadline = defaultAckDeadline
	}
	if cfg.RetentionDuration == 0 {
		cfg.RetentionDuration = defaultRetentionDuration
	}

	created, err := ps.client.CreateSubscription(ctx, subID, cfg)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			logger.Info("Pub/Sub subscription already exists (created concurrently)",
				zap.String("subscription", subID),
				zap.String("topic", topic.ID()),
				zap.Duration("duration", time.Since(start)),
			)
			return sub, nil
		}
		return nil, fmt.Errorf("create subscription %q: %w", subID, err)
	}
	logger.Info("Created Pub/Sub subscription",
		zap.String("subscription", subID),
		zap.String("topic", topic.ID()),
		zap.Duration("duration", time.Since(start)),
	)
	return created, nil
}

func (ps *PubSub) EnsureTopicSubscriptions(ctx context.Context) error {
	logger := logs.GetLogger()
	raw := config.GetMap("pubSub.topicSubscriptions")
	if len(raw) == 0 {
		logger.Info("pubSub.topicSubscriptions not configured; skipping bootstrap")
		return nil
	}

	start := time.Now()
	var topicCount, subCount int
	logger.Info("Bootstrapping Pub/Sub topics and subscriptions",
		zap.Int("topicCount", len(raw)),
	)

	for topicID, subsRaw := range raw {
		subIDs, err := toStringSlice(subsRaw)
		if err != nil {
			return fmt.Errorf("pubSub.topicSubscriptions[%q]: %w", topicID, err)
		}

		topic, err := ps.EnsureTopic(ctx, topicID)
		if err != nil {
			return err
		}
		topicCount++

		for _, subID := range subIDs {
			if _, err := ps.EnsureSubscription(ctx, subID, topic, cloudpubsub.SubscriptionConfig{}); err != nil {
				return err
			}
			subCount++
		}
	}

	logger.Info("Pub/Sub bootstrap complete",
		zap.Int("topics", topicCount),
		zap.Int("subscriptions", subCount),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

func toStringSlice(v interface{}) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	switch s := v.(type) {
	case []string:
		return s, nil
	case []interface{}:
		out := make([]string, 0, len(s))
		for i, item := range s {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("element %d is %T, want string", i, item)
			}
			out = append(out, str)
		}
		return out, nil
	default:
		return nil, errors.New("expected list of subscription names")
	}
}
