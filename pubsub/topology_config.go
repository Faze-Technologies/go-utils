package pubsub

import (
	"context"
	"errors"
	"fmt"

	"github.com/Faze-Technologies/go-utils/config"
)

// EnsureTopologyFromConfig reads `pubSub.topicSubscriptions` and calls EnsureTopology.
// No-op if the key is missing or empty.
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
		return SubscriptionSpec{}, fmt.Errorf("expected {name, filter, enableMessageOrdering} object, got %T", item)
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

	return SubscriptionSpec{Name: name, Filter: filter, EnableMessageOrdering: ordering}, nil
}
