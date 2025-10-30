package rulesengine

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// -------------------- Types --------------------

type Config struct {
	Name       string     `json:"name"`
	RuleConfig RuleHolder `json:"ruleConfig"`
	// Rewards    []Reward   `json:"rewards,omitempty"`
	UpdatedAt  string     `json:"updatedAt,omitempty"`
	updatedAtT time.Time  // parsed UpdatedAt (internal)
}

// type Reward struct {s
// 	Type  string      `json:"type"`
// 	Mode  string      `json:"mode"`
// 	Value interface{} `json:"Value"`
// }

type RuleHolder struct {
	Name       string    `json:"name"`
	Priority   int       `json:"priority"`
	RuleConfig *RuleNode `json:"ruleConfig"`
}

type RuleNode struct {
	// Leaf
	Key   *string     `json:"key,omitempty"`
	Value interface{} `json:"value,omitempty"`

	// Nested
	Operator string      `json:"operator,omitempty"` // "AND" | "OR"
	Rules    []*RuleNode `json:"rules,omitempty"`
}

// Event stores root map and metaData separately for easy lookup
type Event struct {
	Root     map[string]interface{}
	MetaData map[string]interface{}
}

// -------------------- Public API --------------------

// FindMatchingRules: external signature (keeps interface{} inputs).
// Converts inputs into typed structs (using json.Decoder with UseNumber),
// validates configs (immediate error if invalid per Option A), sorts,
// evaluates recursively, and returns the full matched typed Config as interface{} (or error).
func FindMatchingRules(cfgs []interface{}, event interface{}) (interface{}, error) {
	// Convert configs interfaces -> []Config (typed) with Decoder.UseNumber
	configs := make([]Config, 0, len(cfgs))
	for idx, ci := range cfgs {
		// Marshal the interface element to JSON bytes
		b, err := json.Marshal(ci)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal config[%d]: %w", idx, err)
		}
		// Decode with UseNumber into typed Config
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var c Config
		if err := dec.Decode(&c); err != nil {
			return nil, fmt.Errorf("failed to decode config[%d]: %w", idx, err)
		}
		// Validate presence of nested ruleConfig and priority (per Option A)
		if c.RuleConfig.RuleConfig == nil {
			return nil, fmt.Errorf("config %q: missing ruleConfig.ruleConfig", c.Name)
		}
		if c.RuleConfig.Priority == 0 {
			return nil, fmt.Errorf("config %q: missing ruleConfig.priority", c.Name)
		}
		// parse updatedAt if present
		if c.UpdatedAt != "" {
			t, err := time.Parse(time.RFC3339, c.UpdatedAt)
			if err != nil {
				// Option A: return error on invalid updatedAt
				return nil, fmt.Errorf("config %q: invalid updatedAt: %w", c.Name, err)
			}
			c.updatedAtT = t
		} else {
			c.updatedAtT = time.Time{}
		}

		// Validate nested rule nodes for missing operator in nested nodes:
		if err := validateRuleNode(c.RuleConfig.RuleConfig); err != nil {
			return nil, fmt.Errorf("config %q: invalid ruleConfig: %w", c.Name, err)
		}

		configs = append(configs, c)
	}

	// Convert event interface{} -> Event (map + metadata) using Decoder.UseNumber
	evMap, err := toMapWithNumbers(event)
	if err != nil {
		return nil, fmt.Errorf("failed to parse event: %w", err)
	}
	ev := NewEventFromMap(evMap)

	// Sort configs: priority ASC, updatedAt DESC
	sort.SliceStable(configs, func(i, j int) bool {
		pi := configs[i].RuleConfig.Priority
		pj := configs[j].RuleConfig.Priority
		if pi != pj {
			return pi < pj
		}
		// same priority -> newer updatedAt first
		return configs[i].updatedAtT.After(configs[j].updatedAtT)
	})

	// Evaluate each config and return first match
	for _, cfg := range configs {
		ok, err := evaluateRuleNode(cfg.RuleConfig.RuleConfig, ev)
		if err != nil {
			// if evaluation error (should be rare since we validated earlier), treat as failure
			return nil, fmt.Errorf("error evaluating config %q: %w", cfg.Name, err)
		}
		if ok {
			return cfg, nil
		}
	}

	return nil, errors.New("no matching rule config found")
}

// -------------------- Helpers --------------------

// toMapWithNumbers converts unknown interface{} to map[string]interface{}
// while ensuring numbers are json.Number via encoding/json.Decoder.UseNumber.
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

// validateRuleNode checks nested nodes to ensure nested groups have an operator.
// Per your choice A, missing operator in a nested node returns an error.
func validateRuleNode(node *RuleNode) error {
	if node == nil {
		return errors.New("rule node is nil")
	}
	// If this is a nested node (rules present), operator must be present and valid.
	if len(node.Rules) > 0 {
		op := strings.ToUpper(strings.TrimSpace(node.Operator))
		if op != "AND" && op != "OR" {
			return fmt.Errorf("nested rule missing or invalid operator: got %q", node.Operator)
		}
		// validate children
		for _, r := range node.Rules {
			if err := validateRuleNode(r); err != nil {
				return err
			}
		}
		return nil
	}
	// Leaf node: must have Key (we allow Value to be nil but then comparison will be nil)
	if node.Key == nil {
		return errors.New("leaf rule missing key")
	}
	return nil
}

// evaluateRuleNode evaluates a RuleNode against the event, returns (bool, error).
// Errors arise if nested operator is missing (should've been caught in validate, but double-check).
func evaluateRuleNode(node *RuleNode, event Event) (bool, error) {
	if node == nil {
		return false, nil
	}

	// Leaf node
	if node.Key != nil && len(node.Rules) == 0 {
		evVal, found := lookupEventKey(*node.Key, event)
		if !found {
			// per spec: missing key => no match
			return false, nil
		}
		return compareValues(node.Value, evVal), nil
	}

	// Nested node: operator must be provided (we validated earlier but enforce here)
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

// lookupEventKey searches root first, then metaData
func lookupEventKey(key string, event Event) (interface{}, bool) {
	if event.Root != nil {
		if v, ok := event.Root[key]; ok {
			return v, true
		}
	}
	if event.MetaData != nil {
		if v, ok := event.MetaData[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// compareValues compares ruleValue and eventValue.
// - Allows numeric <-> string comparisons if numeric-equivalent.
// - String comparisons are case-sensitive.
// - Uses json.Number parsing where applicable.
func compareValues(ruleValue interface{}, eventValue interface{}) bool {
	// nil checks
	if ruleValue == nil || eventValue == nil {
		return ruleValue == eventValue
	}

	// If either is numeric (json.Number or numeric types or numeric-strings), try numeric compare
	rIsNum, rNum := toNumeric(ruleValue)
	eIsNum, eNum := toNumeric(eventValue)

	if rIsNum && eIsNum {
		return rNum == eNum
	}

	// Fallback to string equality (case-sensitive)
	rStr := toStringValue(ruleValue)
	eStr := toStringValue(eventValue)
	return rStr == eStr
}

// toNumeric attempts to interpret v as numeric and returns (true, float64) if numeric.
// Handles json.Number, numeric types and numeric strings.
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
	case int:
		return true, float64(t)
	case int8:
		return true, float64(t)
	case int16:
		return true, float64(t)
	case int32:
		return true, float64(t)
	case int64:
		return true, float64(t)
	case uint:
		return true, float64(t)
	case uint8:
		return true, float64(t)
	case uint16:
		return true, float64(t)
	case uint32:
		return true, float64(t)
	case uint64:
		return true, float64(t)
	case string:
		// try parse
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
	case float32, float64, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
