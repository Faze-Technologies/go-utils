package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/Faze-Technologies/go-utils/config"
	"go.uber.org/zap"
)

const slackPostMessageURL = "https://slack.com/api/chat.postMessage"

type SlackAlertOptions struct {
	IconEmoji string // e.g. ":robot_face:"
}

func SendSlackAlert(ctx context.Context, logger *zap.Logger, channelName, message, senderName string, opts ...SlackAlertOptions) bool {
	slackToken := config.GetString("SLACK_TOKEN")
	if slackToken == "" {
		logger.Warn("Slack token is not set in  config. Skipping Slack Alert.")
		return false
	}

	payload := map[string]interface{}{
		"channel":  channelName,
		"text":     message,
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
