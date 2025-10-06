package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/slack-go/slack"
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

// PostMessage は指定されたレビュー結果（Markdown）を Slack チャンネルに投稿します。
// Block Kitを使用してリッチなメッセージを構築します。
func (c *SlackClient) PostMessage(markdownText string) error {
	// 1. Block Kitコンポーネントの構築

	// メッセージのヘッダーブロック（タイトル）を作成
	headerBlock := slack.NewHeaderBlock(
		// plain_text を使用し、絵文字を有効にすることでタイトルを強調
		slack.NewTextBlockObject("plain_text", "🤖 Gemini AI Code Review Result:", true, false),
	)

	// Markdown テキストを格納する Section ブロックを作成
	// type: "mrkdwn" を指定することで、入力テキストのMarkdown記法が有効になります。
	sectionBlock := slack.NewSectionBlock(
		// mrkdwnオブジェクトは自動でエスケープ処理を行うため、手動エスケープは不要です
		slack.NewTextBlockObject("mrkdwn", markdownText, false, false),
		nil, // Fields (列) は使用しない
		nil, // Accessory (ボタンなど) は使用しない
	)

	// 複数のブロックを配列にまとめる
	blocks := []slack.Block{headerBlock, sectionBlock}

	// 2. Webhook用のペイロードを構築
	msg := slack.WebhookMessage{
		// 通知用の代替テキスト
		Text: "新しい Gemini AI コードレビュー結果が届きました。",
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

	// 3. JSONペイロードに変換
	jsonPayload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// 4. HTTPリクエスト処理
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
