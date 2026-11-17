package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Faze-Technologies/go-utils/config"
	"go.uber.org/zap"
)

const slackPostMessageURL = "https://slack.com/api/chat.postMessage"

// slackEnvironmentChannels routes alerts outside production to one channel
// per environment, as common-service's sendSlackAlert does, so dev and
// preprod alerts never reach production channels. Production, and an
// environment not listed here, keep the caller's channel.
var slackEnvironmentChannels = map[string]string{
	"dev":     "env-dev-alerts",
	"sgdev":   "env-sgdev-alerts",
	"stage":   "env-staging-alerts",
	"staging": "env-staging-alerts",
	"preprod": "env-preprod-alerts",
}

type SlackAlertOptions struct {
	IconEmoji string // e.g. ":robot_face:"
	// KeepChannel sends to channelName in every environment, for a caller
	// that already picks its own non-production channel.
	KeepChannel bool
}

// slackAlertChannel is the channel an alert goes to in environment env.
func slackAlertChannel(env, channelName string, keepChannel bool) string {
	if keepChannel {
		return channelName
	}
	if channel, ok := slackEnvironmentChannels[strings.ToLower(strings.TrimSpace(env))]; ok {
		return channel
	}
	return channelName
}

// slackAlertText names the sending service and its environment at the top
// of the message, so an alert in a shared channel says where it came from.
func slackAlertText(serviceName, env, message string) string {
	if serviceName == "" {
		return message
	}
	if env = strings.TrimSpace(env); env != "" {
		return fmt.Sprintf("Message from `%s` (%s)\n%s", serviceName, env, message)
	}
	return fmt.Sprintf("Message from `%s`\n%s", serviceName, message)
}

// SendSlackAlert posts message to Slack and reports whether Slack accepted
// it. Outside production the alert goes to that environment's channel (see
// slackEnvironmentChannels) unless KeepChannel is set, and the message is
// prefixed with the service and environment it came from.
func SendSlackAlert(ctx context.Context, logger *zap.Logger, channelName, message, senderName string, opts ...SlackAlertOptions) bool {
	slackToken := config.GetString("SLACK_TOKEN")
	if slackToken == "" {
		logger.Warn("Slack token is not set in  config. Skipping Slack Alert.")
		return false
	}

	env := config.GetString("environment")
	keepChannel := len(opts) > 0 && opts[0].KeepChannel
	payload := map[string]interface{}{
		"channel":  slackAlertChannel(env, channelName, keepChannel),
		"text":     slackAlertText(config.GetString("serviceName"), env, message),
		"username": senderName,
	}
	if len(opts) > 0 && opts[0].IconEmoji != "" {
		payload["icon_emoji"] = opts[0].IconEmoji
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logger.Error("sendSlackAlert: failed to marshal payload", zap.Error(err))
		return false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackPostMessageURL, bytes.NewReader(body))
	if err != nil {
		logger.Error("sendSlackAlert: failed to create request", zap.Error(err))
		return false
	}
	req.Header.Set("Authorization", "Bearer "+slackToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("sendSlackAlert: request failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("sendSlackAlert: non-200 HTTP response from Slack",
			zap.Int("statusCode", resp.StatusCode),
		)
		return false
	}

	var slackResp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&slackResp); err != nil {
		logger.Warn("sendSlackAlert: failed to decode Slack response", zap.Error(err))
		return false
	}
	if !slackResp.OK {
		logger.Error("sendSlackAlert: Slack API returned error", zap.String("error", slackResp.Error))
		return false
	}
	return true
}
