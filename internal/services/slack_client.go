package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
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

// extractRepoPath は、HTTP/HTTPSまたはSSH形式のGit URLから 'owner/repo' 形式のパスを抽出します。
func extractRepoPath(gitCloneURL string) string {
	// 1. SSH形式のURL (git@host:owner/repo.git) の処理
	// 例: git@github.com:nabeken/blackfriday-slack-block-kit.git
	// 正規表現で `:owner/repo.git` の部分を抽出
	reSSH := regexp.MustCompile(`:([a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+)\.git$`)
	if strings.HasPrefix(gitCloneURL, "git@") {
		matches := reSSH.FindStringSubmatch(gitCloneURL)
		if len(matches) == 2 {
			// matches[1] が 'owner/repo' に相当
			return matches[1]
		}
	}

	// 2. HTTP/HTTPS形式のURLの処理
	// 例: https://github.com/owner/repo.git または https://gitlab.com/owner/repo
	parsedURL, err := url.Parse(gitCloneURL)
	if err == nil && parsedURL.Host != "" {
		// パスから '.git' サフィックスを削除し、空の要素をフィルタリング
		path := strings.TrimSuffix(parsedURL.Path, ".git")
		parts := strings.Split(path, "/")

		// 空の要素（先頭のスラッシュなど）を取り除く
		var cleanParts []string
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}

		if len(cleanParts) >= 2 {
			// owner/repo の形式であれば、後ろ2つを結合
			return cleanParts[len(cleanParts)-2] + "/" + cleanParts[len(cleanParts)-1]
		}
		if len(cleanParts) == 1 {
			// /repo の形式であればそれを使用
			return cleanParts[0]
		}
	}

	// 3. どちらにもマッチしない場合は、単にブランチ名のみを通知に使うため、"リポジトリ"というプレースホルダーを返す
	return "リポジトリ"
}

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
func (c *SlackClient) PostMessage(markdownText string, featureBranch string, gitCloneURL string) error {

	// 1. 通知テキストの生成 (extractRepoPath を利用)
	repoPath := extractRepoPath(gitCloneURL)

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
		nil, // Fields (列) は使用しない
		nil, // Accessory (ボタンなど) は使用しない
	)

	// 複数のブロックを配列にまとめる
	blocks := []slack.Block{headerBlock, sectionBlock}

	// 3. Webhook用のペイロードを構築
	msg := slack.WebhookMessage{
		// 修正後の動的な通知テキストを設定
		Text: notificationText,
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

	// 4. JSONペイロードに変換
	jsonPayload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// 5. HTTPリクエスト処理
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
