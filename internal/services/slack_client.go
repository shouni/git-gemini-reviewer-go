package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
func (c *SlackClient) PostMessage(markdownText string, featureBranch string, gitCloneURL string) error {

	// 1. 通知テキストの生成
	// リポジトリ名 (例: owner/repo) を URL から抽出するシンプルなロジック
	repoPath := gitCloneURL
	if strings.Contains(repoPath, "/") {
		// URLからホストと拡張子を除去
		parts := strings.Split(strings.TrimSuffix(repoPath, ".git"), "/")
		if len(parts) >= 2 {
			repoPath = parts[len(parts)-2] + "/" + parts[len(parts)-1]
		}
	} else {
		// URLが不完全な場合は、ブランチ名のみを使用
		repoPath = "リポジトリ"
	}

	// 通知用の代替テキストを構築
	notificationText := fmt.Sprintf(
		"✅ Gemini AI レビュー完了: `%s` ブランチ (%s)",
		featureBranch,
		repoPath,
	)

	// 2. Block Kitコンポーネントの構築
	headerBlock := slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", "🤖 Gemini AI Code Review Result:", true, false),
	)

	sectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", markdownText, false, false),
		nil,
		nil,
	)

	// 複数のブロックを配列にまとめる
	blocks := []slack.Block{headerBlock, sectionBlock}

	// 3. Webhook用のペイロードを構築
	msg := slack.WebhookMessage{
		Text: notificationText,
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

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
