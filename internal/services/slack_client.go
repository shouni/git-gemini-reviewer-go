package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// SlackClient は Slack API と連携するためのクライアントです。
type SlackClient struct {
	WebhookURL string
}

// NewSlackClient は SlackClient の新しいインスタンスを作成します。
func NewSlackClient(webhookURL string) *SlackClient {
	return &SlackClient{
		WebhookURL: webhookURL,
	}
}

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
func (c *SlackClient) PostMessage(channelID, text string) error {
	// レビュー結果をマークダウン形式のテキストとして整形
	payload := map[string]string{
		"text": fmt.Sprintf("*🤖 Gemini AI Code Review Result:*\n\n%s", text),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	resp, err := http.Post(c.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to post to Slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned non-OK status code: %s", resp.Status)
	}

	return nil
}

