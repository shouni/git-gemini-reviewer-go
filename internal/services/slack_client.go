package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log" // log パッケージを追加
	"net/http"
	"time" // time パッケージを追加
)

// SlackClient は Slack API と連携するためのクライアントです。
type SlackClient struct {
	WebhookURL string
	httpClient *http.Client // カスタムHTTPクライアント
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

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
// Webhook API を使用するため、channelID 引数は削除しました。
func (c *SlackClient) PostMessage(text string) error {
	payload := map[string]string{
		"text": fmt.Sprintf("*🤖 Gemini AI Code Review Result:*\n\n%s", text),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// カスタムクライアントを使用し、リクエストを送信
	resp, err := c.httpClient.Post(c.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to post to Slack: %w", err)
	}

	// defer で Body.Close() のエラーを処理
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
