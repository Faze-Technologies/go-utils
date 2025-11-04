package rulesengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

// -------------------- Types --------------------

type RuleConfigs struct {
	Name       string     `json:"name" bson:"name"`
	RuleConfig RuleHolder `json:"ruleConfig" bson:"ruleConfig"`
	Rewards    []Reward   `json:"rewards,omitempty" bson:"rewards,omitempty"`
	CreatedAt  time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt" bson:"updatedAt"`
}

type Reward struct {
	Type  string      `json:"type" bson:"type"`
	Mode  string      `json:"mode" bson:"mode"`
	Value interface{} `json:"value" bson:"value"`
}

type RuleHolder struct {
	Name     string   `json:"name" bson:"name"`
	Priority int      `json:"priority" bson:"priority"`
	RuleNode RuleNode `json:"ruleNode" bson:"ruleNode"`
}

type RuleNode struct {
	Key      *string     `json:"key,omitempty" bson:"key,omitempty"`
	Value    interface{} `json:"value,omitempty" bson:"value,omitempty"`
	Operator string      `json:"operator,omitempty" bson:"operator,omitempty"`
	Rules    []*RuleNode `json:"rules,omitempty" bson:"rules,omitempty"`
}

// -------------------- Event --------------------

type Event struct {
	Root     map[string]interface{}
	MetaData map[string]interface{}
}

// -------------------- Public API --------------------

// FindMatchingRules evaluates a list of configs against an event and returns the first matching config.
func FindMatchingRules(cfgs []interface{}, event interface{}) ([]RuleConfigs, error) {
	configs := make([]RuleConfigs, 0, len(cfgs))

	// --- Parse and validate all configs ---
	for idx, ci := range cfgs {
		b, err := json.Marshal(ci)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config[%d]: %w", idx, err)
		}

		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()

		var c RuleConfigs
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("failed to decode config[%d]: %w", idx, err)
		}

		// Basic validations
		if len(c.RuleConfig.RuleNode.Rules) == 0 && c.RuleConfig.RuleNode.Key == nil {
			return nil, fmt.Errorf("config %q: missing ruleConfig.ruleNode", c.Name)
		}
		if c.RuleConfig.Priority == 0 {
			return nil, fmt.Errorf("config %q: missing ruleConfig.priority", c.Name)
		}
		if err := validateRuleNode(&c.RuleConfig.RuleNode); err != nil {
			return nil, fmt.Errorf("config %q: invalid ruleConfig: %w", c.Name, err)
		}

		configs = append(configs, c)
	}

	// --- Parse event into Event struct ---
	evMap, err := toMapWithNumbers(event)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}
	ev := NewEventFromMap(evMap)

	// --- Filter matching configs ---
	matched := make([]RuleConfigs, 0)
	for _, cfg := range configs {
		ok, err := evaluateRuleNode(&cfg.RuleConfig.RuleNode, ev)
		if err != nil {
			return nil, fmt.Errorf("error evaluating config %q: %w", cfg.Name, err)
		}
		if ok {
			matched = append(matched, cfg)
		}
	}

	if len(matched) == 0 {
		return nil, errors.New("no matching rule config found")
	}

	// --- Sort matched configs ---
	sort.SliceStable(matched, func(i, j int) bool {
		pi := matched[i].RuleConfig.Priority
		pj := matched[j].RuleConfig.Priority
		if pi != pj {
			return pi < pj
		}
		if matched[i].UpdatedAt.Equal(matched[j].UpdatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})

	return matched, nil
}

// -------------------- Helpers --------------------

func toMapWithNumbers(v interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var m map[string]interface{}
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func NewEventFromMap(m map[string]interface{}) Event {
	ev := Event{
		Root:     m,
		MetaData: map[string]interface{}{},
	}
	if md, ok := m["metaData"]; ok {
		if mdmap, ok2 := md.(map[string]interface{}); ok2 {
			ev.MetaData = mdmap
		}
	}
	return ev
}

// -------------------- Validation --------------------

func validateRuleNode(node *RuleNode) error {
	if node == nil {
		return errors.New("rule node is nil")
	}

	if len(node.Rules) > 0 {
		op := strings.ToUpper(strings.TrimSpace(node.Operator))
		if op != "AND" && op != "OR" {
			return fmt.Errorf("nested rule missing or invalid operator: got %q", node.Operator)
		}
		for _, r := range node.Rules {
			if err := validateRuleNode(r); err != nil {
				return err
			}
		}
		return nil
	}

	if node.Key == nil {
		return errors.New("leaf rule missing key")
	}
	return nil
}

// -------------------- Evaluation --------------------

func evaluateRuleNode(node *RuleNode, event Event) (bool, error) {
	if node == nil {
		return false, nil
	}

	// Leaf node
	if node.Key != nil && len(node.Rules) == 0 {
		evVal, found := lookupEventKey(*node.Key, event)
		if !found {
			return false, nil
		}
		return compareValues(node.Value, evVal), nil
	}

	// Nested node
	if len(node.Rules) == 0 {
		return false, nil
	}

	op := strings.ToUpper(strings.TrimSpace(node.Operator))
	if op != "AND" && op != "OR" {
		return false, fmt.Errorf("nested node has invalid operator: %q", node.Operator)
	}

	switch op {
	case "AND":
		for _, r := range node.Rules {
			ok, err := evaluateRuleNode(r, event)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case "OR":
		for _, r := range node.Rules {
			ok, err := evaluateRuleNode(r, event)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, nil
	}
}

// -------------------- Value Comparison --------------------

func lookupEventKey(key string, event Event) (interface{}, bool) {
	// Handle dot notation for nested keys
	if strings.Contains(key, ".") {
		return lookupNestedKey(key, event.Root)
	}

	// First check root level
	if event.Root != nil {
		if v, ok := event.Root[key]; ok {
			return v, true
		}
	}

	// Then check metadata
	if event.MetaData != nil {
		if v, ok := event.MetaData[key]; ok {
			return v, true
		}
	}

	return nil, false
}

func lookupNestedKey(key string, data map[string]interface{}) (interface{}, bool) {
	keys := strings.Split(key, ".")
	current := data

	for i, k := range keys {
		if current == nil {
			return nil, false
		}

		val, exists := current[k]
		if !exists {
			return nil, false
		}

		// If this is the last key, return the value
		if i == len(keys)-1 {
			return val, true
		}

		// Otherwise, continue traversing
		if nextMap, ok := val.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil, false
		}
	}

	return nil, false
}

func compareValues(ruleValue interface{}, eventValue interface{}) bool {
	if ruleValue == nil || eventValue == nil {
		return ruleValue == eventValue
	}

	rIsNum, rNum := toNumeric(ruleValue)
	eIsNum, eNum := toNumeric(eventValue)
	if rIsNum && eIsNum {
		return rNum == eNum
	}

	rStr := toStringValue(ruleValue)
	eStr := toStringValue(eventValue)
	return rStr == eStr
}

func toNumeric(v interface{}) (bool, float64) {
	switch t := v.(type) {
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return true, f
		}
		if i, err2 := t.Int64(); err2 == nil {
			return true, float64(i)
		}
		return false, 0
	case *json.Number:
		if t == nil {
			return false, 0
		}
		f, err := t.Float64()
		if err == nil {
			return true, f
		}
		if i, err2 := t.Int64(); err2 == nil {
			return true, float64(i)
		}
		return false, 0
	case float32:
		return true, float64(t)
	case float64:
		return true, t
	case int, int8, int16, int32, int64:
		return true, float64(reflect.ValueOf(t).Int())
	case uint, uint8, uint16, uint32, uint64:
		return true, float64(reflect.ValueOf(t).Uint())
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return true, f
		}
		return false, 0
	default:
		return false, 0
	}
}

func toStringValue(v interface{}) string {
	switch t := v.(type) {
	case json.Number:
		return t.String()
	case *json.Number:
		if t == nil {
			return ""
		}
		return t.String()
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
