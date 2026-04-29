package pubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/Faze-Technologies/go-utils/config"
)

// EnsureTopologyFromConfig is the config-driven entry point: reads
// `pubSub.topicSubscriptions` from the loaded config and forwards to
// EnsureTopology. Missing or empty config is a silent no-op so services
// without a pubsub topology don't have to special-case the call.
func (ps *PubSub) EnsureTopologyFromConfig(ctx context.Context) error {
	topology, err := loadTopologyFromConfig()
	if err != nil {
		return err
	}
	return ps.EnsureTopology(ctx, topology)
}

func loadTopologyFromConfig() ([]TopicSpec, error) {
	raw := config.GetMap("pubSub.topicSubscriptions")
	if len(raw) == 0 {
		return nil, nil
	}

	topology := make([]TopicSpec, 0, len(raw))
	for topicName, subsRaw := range raw {
		subs, err := toSubscriptionSpecs(subsRaw)
		if err != nil {
			return nil, fmt.Errorf("pubSub.topicSubscriptions[%q]: %w", topicName, err)
		}
		topology = append(topology, TopicSpec{
			Name:          topicName,
			Subscriptions: subs,
		})
	}
	return topology, nil
}

func toSubscriptionSpecs(v interface{}) ([]SubscriptionSpec, error) {
	if v == nil {
		return nil, nil
	}
	items, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected list of subscription objects, got %T", v)
	}
	out := make([]SubscriptionSpec, 0, len(items))
	for i, item := range items {
		spec, err := toSubscriptionSpec(item)
		if err != nil {
			return nil, fmt.Errorf("element %d: %w", i, err)
		}
		out = append(out, spec)
	}
	return out, nil
}

func toSubscriptionSpec(item interface{}) (SubscriptionSpec, error) {
	obj, ok := item.(map[string]interface{})
	if !ok {
		return SubscriptionSpec{}, fmt.Errorf("expected subscription object, got %T", item)
	}

	nameRaw, ok := obj["name"]
	if !ok {
		return SubscriptionSpec{}, errors.New(`missing "name"`)
	}
	name, ok := nameRaw.(string)
	if !ok || name == "" {
		return SubscriptionSpec{}, fmt.Errorf(`"name" must be a non-empty string, got %T`, nameRaw)
	}

	var filter string
	if filterRaw, ok := obj["filter"]; ok && filterRaw != nil {
		filter, ok = filterRaw.(string)
		if !ok {
			return SubscriptionSpec{}, fmt.Errorf(`"filter" must be a string, got %T`, filterRaw)
		}
	}

	var ordering bool
	if orderingRaw, ok := obj["enableMessageOrdering"]; ok && orderingRaw != nil {
		ordering, ok = orderingRaw.(bool)
		if !ok {
			return SubscriptionSpec{}, fmt.Errorf(`"enableMessageOrdering" must be a bool, got %T`, orderingRaw)
		}
	}

	// JSON numbers decode as float64 through encoding/json; YAML loaders
	// (viper) sometimes hand back int directly. Accept both rather than
	// forcing callers to know which path their config went through.
	var maxDeliveryAttempts int
	if mdaRaw, ok := obj["maxDeliveryAttempts"]; ok && mdaRaw != nil {
		switch v := mdaRaw.(type) {
		case float64:
			maxDeliveryAttempts = int(v)
		case int:
			maxDeliveryAttempts = v
		default:
			return SubscriptionSpec{}, fmt.Errorf(`"maxDeliveryAttempts" must be a number, got %T`, mdaRaw)
		}
	}

	var deadLetterTopic string
	if dltRaw, ok := obj["deadLetterTopic"]; ok && dltRaw != nil {
		deadLetterTopic, ok = dltRaw.(string)
		if !ok {
			return SubscriptionSpec{}, fmt.Errorf(`"deadLetterTopic" must be a string, got %T`, dltRaw)
		}
	}

	return SubscriptionSpec{
		Name:                  name,
		Filter:                filter,
		EnableMessageOrdering: ordering,
		MaxDeliveryAttempts:   maxDeliveryAttempts,
		DeadLetterTopic:       deadLetterTopic,
	}, nil
}
