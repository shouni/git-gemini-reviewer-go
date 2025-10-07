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
			Timeout: 10 * time.Second,
		},
	}
}

// getRepoIdentifier は、Git URLから 'owner/repo' 形式のパスを抽出します。
func getRepoIdentifier(gitCloneURL string) string {
	reSSH := regexp.MustCompile(`:([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)\.git$`)
	if strings.HasPrefix(gitCloneURL, "git@") {
		matches := reSSH.FindStringSubmatch(gitCloneURL)
		if len(matches) == 2 {
			return matches[1]
		}
	}
	parsedURL, err := url.Parse(gitCloneURL)
	if err != nil {
		log.Printf("WARNING: Failed to parse Git clone URL '%s': %v", gitCloneURL, err)
		return ""
	}
	if parsedURL.Host != "" {
		path := strings.TrimSuffix(parsedURL.Path, ".git")
		parts := strings.Split(path, "/")
		var cleanParts []string
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}
		if len(cleanParts) == 2 {
			return cleanParts[0] + "/" + cleanParts[1]
		}
	}
	return ""
}

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
func (c *SlackClient) PostMessage(markdownText string, featureBranch string, gitCloneURL string) error {

	// Slack Section Block内のmrkdwnテキストの最大文字数は3000文字
	const maxMrkdwnLength = 3000
	const suffix = "\n\n...(レビュー結果が長すぎたため、一部省略されました)"

	// 1. レビュー結果のテキストを Slack 向けに整形
	// Slackは # や ## を認識しないため、太字(*)と水平線(---)に変換
	postableText := strings.ReplaceAll(markdownText, "## ", "*")
	postableText = strings.ReplaceAll(postableText, "---", "\n---\n")

	// 文字数チェックと切り詰め
	if len(postableText) > maxMrkdwnLength {
		log.Printf("WARNING: Markdown text length (%d chars) exceeds Block Kit limit (%d chars). Truncating message.", len(postableText), maxMrkdwnLength)
		truncateLength := maxMrkdwnLength - len(suffix)
		postableText = postableText[:truncateLength] + suffix
	}

	// 2. Block Kitコンポーネントの構築
	headerBlock := slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", "🤖 Gemini AI Code Review Result:", true, false),
	)

	// ブランチ名とリポジトリ名を表示するセクションブロック
	branchSectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("✅ *Gemini AI レビュー完了*: `%s` ブランチ (%s)", featureBranch, getRepoIdentifier(gitCloneURL)), false, false),
		nil,
		nil,
	)

	// レビュー結果のコンテンツセクションブロック
	contentSectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", postableText, false, false),
		nil,
		nil,
	)

	// 複数のブロックを配列にまとめる
	blocks := []slack.Block{headerBlock, branchSectionBlock, contentSectionBlock}

	// 3. Webhook用のペイロードを構築
	msg := slack.WebhookMessage{
		// Blocksがあるため、Textはプレビュー用。今回は空でも問題ない。
		Text:   "",
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
