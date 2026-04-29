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

// Defaults are code constants, not env config: changing them per environment
// invites subtle bugs (e.g. a short ack deadline causing redeliveries mid-handler).
const (
	defaultAckDeadline       = 60 * time.Second
	defaultRetentionDuration = 7 * 24 * time.Hour
	defaultMinBackoff        = 10 * time.Second
	defaultMaxBackoff        = 600 * time.Second
	// dlqTopicSuffix is appended to a subscription's name to derive its
	// per-subscription DLQ topic when DeadLetterTopic isn't set explicitly.
	// Per-sub DLQs (vs one shared DLQ) keep triage tractable.
	dlqTopicSuffix = "-dlq"
)

type SubscriptionSpec struct {
	Name string
	// Filter is a Pub/Sub native filter expression on message attributes
	// (e.g. `attributes.region = "us"`). Body content is not visible to
	// the filter. Immutable after subscription creation — to change it,
	// give the subscription a new name and delete the old.
	Filter string
	// EnableMessageOrdering also requires the publisher to set OrderingKey
	// on each message; otherwise ordering is best-effort only.
	EnableMessageOrdering bool
	// MaxDeliveryAttempts ∈ [5,100] enables the DLQ. Zero (unset) means
	// no DLQ. Below 5 or above 100 GCP rejects at create-time.
	MaxDeliveryAttempts int
	// DeadLetterTopic overrides the auto-named "<Name>-dlq" target.
	// Only set when sharing a DLQ across subscriptions is intentional.
	//
	// IMPORTANT — IAM is NOT granted by EnsureTopology. The GCP-managed
	// Pub/Sub service account
	// (service-<projectNumber>@gcp-sa-pubsub.iam.gserviceaccount.com)
	// needs `roles/pubsub.publisher` on the DLQ topic AND
	// `roles/pubsub.subscriber` on the source subscription, or the DLQ
	// silently does nothing — failed messages just keep redelivering.
	// Provision once per env via Terraform / `gcloud add-iam-policy-binding`.
	DeadLetterTopic string
}

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

// EnsureTopology is create-only and idempotent: existing topics and
// subscriptions are left as-is, never updated. This means changes to mutable
// fields (AckDeadline, RetentionDuration, DeadLetterPolicy, RetryPolicy) on
// already-existing subscriptions DO NOT take effect — recreate the
// subscription out-of-band to apply them.
//
// Filters and ordering are immutable in Pub/Sub itself, so to rotate either,
// pick a new subscription name and delete the old one.
//
// Defaults applied to every new subscription: exactly-once delivery,
// 10s..600s exponential retry. DLQ is only attached when the spec sets
// MaxDeliveryAttempts > 0 — see the IAM caveat on SubscriptionSpec.
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
			if s.MaxDeliveryAttempts > 0 {
				// DLQ topic is auto-created if it doesn't exist — same
				// idempotency rules as any other topic. The fully-qualified
				// "projects/<project>/topics/<name>" form is required by the
				// GCP API; a bare topic ID would be rejected.
				dlqName := s.DeadLetterTopic
				if dlqName == "" {
					dlqName = s.Name + dlqTopicSuffix
				}
				if _, err := ps.EnsureTopic(ctx, dlqName); err != nil {
					return fmt.Errorf("ensure DLQ topic %q for subscription %q: %w", dlqName, s.Name, err)
				}
				cfg.DeadLetterPolicy = &cloudpubsub.DeadLetterPolicy{
					DeadLetterTopic:     fmt.Sprintf("projects/%s/topics/%s", ps.client.Project(), dlqName),
					MaxDeliveryAttempts: s.MaxDeliveryAttempts,
				}
				logger.Info("Subscription will use DLQ",
					zap.String("subscription", s.Name),
					zap.String("dlqTopic", dlqName),
					zap.Int("maxDeliveryAttempts", s.MaxDeliveryAttempts),
				)
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
