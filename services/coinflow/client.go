package coinflow

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/Faze-Technologies/go-utils/apm"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

const defaultBaseURL = "https://api.coinflow.cash"

type Config struct {
	Enabled bool
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

type EventClient struct {
	logger  *zap.Logger
	client  *resty.Client
	enabled bool
	baseURL string
	apiKey  string
}

func NewEventClient(cfg Config, logger *zap.Logger) *EventClient {
	if logger == nil {
		logger = zap.NewNop()
	}

	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	return &EventClient{
		logger: logger,
		client: resty.New().
			SetTransport(apm.NewHTTPTransport()).
			SetTimeout(timeout).
			SetRetryCount(0).
			SetHeader("Content-Type", "application/json"),
		enabled: cfg.Enabled,
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
	}
}

func (c *EventClient) PostEvent(ctx context.Context, payload interface{}) error {
	if c == nil || !c.enabled {
		return nil
	}

	eventType := eventField(payload, "eventType", "EventType")
	customerID := eventField(payload, "customerId", "CustomerID")

	if strings.TrimSpace(c.apiKey) == "" {
		c.logger.Warn("Coinflow event skipped: missing api key", zap.String("eventType", eventType))
		return nil
	}
	if strings.TrimSpace(eventType) == "" {
		return nil
	}

	resp, err := c.client.R().
		SetContext(ctx).
		SetHeader("Authorization", c.apiKey).
		SetBody(payload).
		Post(fmt.Sprintf("%s/api/events", c.baseURL))
	if err != nil {
		c.logger.Warn("Coinflow event request failed",
			zap.String("eventType", eventType),
			zap.String("customerId", customerID),
			zap.Error(err))
		return err
	}
	if resp.IsError() {
		err := fmt.Errorf("coinflow event status %d: %s", resp.StatusCode(), string(resp.Body()))
		c.logger.Warn("Coinflow event returned error",
			zap.String("eventType", eventType),
			zap.String("customerId", customerID),
			zap.Error(err))
		return err
	}

	c.logger.Info("Coinflow event tracked",
		zap.String("eventType", eventType),
		zap.String("customerId", customerID))
	return nil
}

func eventField(payload interface{}, names ...string) string {
	if payload == nil {
		return ""
	}

	switch p := payload.(type) {
	case map[string]interface{}:
		for _, name := range names {
			if value, ok := p[name]; ok {
				return fmt.Sprint(value)
			}
		}
	case map[string]string:
		for _, name := range names {
			if value, ok := p[name]; ok {
				return value
			}
		}
	}

	value := reflect.ValueOf(payload)
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return ""
	}

	for _, name := range names {
		field := value.FieldByName(name)
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}

	return ""
}
