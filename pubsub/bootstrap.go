package pubsub

import (
	"context"
	"fmt"
	"time"

	cloudpubsub "cloud.google.com/go/pubsub"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Faze-Technologies/go-utils/logs"
)

const (
	defaultAckDeadline       = 60 * time.Second
	defaultRetentionDuration = 7 * 24 * time.Hour
	defaultMinBackoff        = 10 * time.Second
	defaultMaxBackoff        = 600 * time.Second
)

// SubscriptionSpec declares one subscription on a topic.
//
// Filter is an optional Pub/Sub filter expression evaluated against
// message attributes (e.g. `attributes.eventType = "order.created"`).
// It is set at creation time and is immutable afterwards.
//
// EnableMessageOrdering, when true, makes the subscription deliver
// messages sharing the same OrderingKey in the order they were
// published. The publisher must also set OrderingKey on each message.
type SubscriptionSpec struct {
	Name                  string
	Filter                string
	EnableMessageOrdering bool
}

// TopicSpec declares one topic and the subscriptions that should exist on it.
type TopicSpec struct {
	Name                  string
	EnableMessageOrdering bool
	Subscriptions         []SubscriptionSpec
}

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

// EnsureTopology ensures every topic and subscription in the given topology
// exists in GCP. It is idempotent — already-existing topics and subscriptions
// are left untouched (Pub/Sub does not allow filter or ordering changes after
// creation, so to rotate one, give it a new name and delete the old one).
//
// All subscriptions are created with exactly-once delivery and an exponential
// retry policy (10s..600s) by default.
func (ps *PubSub) EnsureTopology(ctx context.Context, topology []TopicSpec) error {
	logger := logs.GetLogger()
	if len(topology) == 0 {
		logger.Info("Pub/Sub topology is empty; skipping bootstrap")
		return nil
	}

	start := time.Now()
	var subCount int
	logger.Info("Bootstrapping Pub/Sub topics and subscriptions",
		zap.Int("topicCount", len(topology)),
	)

	for _, t := range topology {
		if t.Name == "" {
			return fmt.Errorf("topology entry has empty topic name")
		}

		topic, err := ps.EnsureTopic(ctx, t.Name)
		if err != nil {
			return err
		}
		if t.EnableMessageOrdering {
			topic.EnableMessageOrdering = true
		}

		for _, s := range t.Subscriptions {
			if s.Name == "" {
				return fmt.Errorf("topic %q: subscription has empty name", t.Name)
			}
			cfg := cloudpubsub.SubscriptionConfig{
				EnableExactlyOnceDelivery: true,
				EnableMessageOrdering:     s.EnableMessageOrdering,
				Filter:                    s.Filter,
				RetryPolicy: &cloudpubsub.RetryPolicy{
					MinimumBackoff: defaultMinBackoff,
					MaximumBackoff: defaultMaxBackoff,
				},
			}
			if _, err := ps.EnsureSubscription(ctx, s.Name, topic, cfg); err != nil {
				return err
			}
			subCount++
		}
	}

	logger.Info("Pub/Sub bootstrap complete",
		zap.Int("topics", len(topology)),
		zap.Int("subscriptions", subCount),
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}
