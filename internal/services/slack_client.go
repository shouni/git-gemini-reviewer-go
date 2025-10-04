package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// SlackClient は Slack API と連携するためのクライアントです。
type SlackClient struct {
	WebhookURL string
	httpClient *http.Client
}

// NewSlackClient は SlackClient の新しいインスタンスを作成します。
func NewSlackClient(webhookURL string) *SlackClient {
	return &SlackClient{
		WebhookURL: webhookURL,
		httpClient: &http.Client{
			// ネットワークのハングアップを防ぐため、10秒のタイムアウトを設定
			Timeout: 10 * time.Second,
		},
	}
}

// slackEscapeText は、Slackのmrkdwn内で特別な意味を持つ文字 (&, <, >) をエスケープします。
// その他のMarkdown文字（*, _, ~など）はSlackが自動で処理します。
func slackEscapeText(text string) string {
	// 参照: https://api.slack.com/reference/messaging/payload#markdown
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
func (c *SlackClient) PostMessage(text string) error {
	// Slackのメッセージ制限に対応 (4000文字だが、安全のため余裕を持たせる)
	const maxSlackMessageLength = 3500
	const prefix = "*🤖 Gemini AI Code Review Result:*\n\n"
	const suffix = "\n\n...(メッセージが長すぎたため一部省略されました)"

	// エスケープ処理を適用
	escapedText := slackEscapeText(text)

	formattedText := prefix + escapedText

	// メッセージが長すぎる場合の処理 (切り詰め)
	if len(formattedText) > maxSlackMessageLength {
		log.Printf("WARNING: Slack message length (%d chars) exceeds recommended limit (%d chars). Truncating message.", len(formattedText), maxSlackMessageLength)

		// プレフィックスの長さ + サフィックスの長さを考慮して切り詰める位置を決定
		truncateLength := maxSlackMessageLength - len(suffix)

		// プレフィックスと切り詰められたテキスト本体、サフィックスを結合
		formattedText = formattedText[:truncateLength] + suffix
	}

	payload := map[string]string{
		"text": formattedText,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// HTTPリクエスト処理
	resp, err := c.httpClient.Post(c.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to post to Slack: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("WARNING: failed to close Slack API response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned non-OK status code: %s", resp.Status)
	}

	return nil
}
