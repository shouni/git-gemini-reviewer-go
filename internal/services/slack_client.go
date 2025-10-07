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

// getRepoIdentifier は、Git URLから 'owner/repo' 形式のパスを抽出します。
// 抽出に失敗した場合は空文字列 ("") を返します。デフォルト値の設定は呼び出し元が行います。
func getRepoIdentifier(gitCloneURL string) string {

	// 1. SSH特殊形式のURL (git@host:owner/repo.git) の処理
	// 修正1: 正規表現にピリオドを許容するよう調整 [a-zA-Z0-9_.-]+
	reSSH := regexp.MustCompile(`:([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)\.git$`)
	if strings.HasPrefix(gitCloneURL, "git@") {
		matches := reSSH.FindStringSubmatch(gitCloneURL)
		if len(matches) == 2 {
			// matches[1] が 'owner/repo' に相当
			return matches[1]
		}
	}

	// 2. HTTP/HTTPS および SSH URL形式 (ssh://host/owner/repo.git) の処理
	parsedURL, err := url.Parse(gitCloneURL)

	// 修正2: URLパースのエラーをログに記録
	if err != nil {
		log.Printf("WARNING: Failed to parse Git clone URL '%s': %v", gitCloneURL, err)
		return "" // エラー発生時は空文字列を返す
	}

	if parsedURL.Host != "" {

		// パスから '.git' サフィックスを削除
		path := strings.TrimSuffix(parsedURL.Path, ".git")
		parts := strings.Split(path, "/")

		// 空の要素（先頭のスラッシュなど）を取り除く
		var cleanParts []string
		for _, part := range parts {
			if part != "" {
				cleanParts = append(cleanParts, part)
			}
		}

		// 一般的な owner/repo 形式 (つまり2つのセグメント) が確認できた場合のみ返す
		if len(cleanParts) == 2 {
			// cleanParts = [owner, repo] の場合
			return cleanParts[0] + "/" + cleanParts[1]
		}
	}

	// どちらにもマッチしない場合は空文字列を返す
	return ""
}

// PostMessage は指定されたレビュー結果を Slack チャンネルに投稿します。
func (c *SlackClient) PostMessage(markdownText string, featureBranch string, gitCloneURL string) error {

	// Slack Section Block内のmrkdwnテキストの最大文字数は3000文字
	const maxMrkdwnLength = 3000
	const suffix = "\n\n...(レビュー結果が長すぎたため、一部省略されました)"

	// 処理対象となる Markdown テキスト
	postableText := markdownText

	// 文字数チェックと切り詰め
	if len(postableText) > maxMrkdwnLength {
		log.Printf("WARNING: Markdown text length (%d chars) exceeds Block Kit limit (%d chars). Truncating message.", len(postableText), maxMrkdwnLength)

		// サフィックスの長さを考慮して切り詰める位置を決定
		truncateLength := maxMrkdwnLength - len(suffix)

		// テキストを切り詰め、サフィックスを結合
		postableText = postableText[:truncateLength] + suffix
	}

	// 1. 通知テキストの生成
	// 修正3: getRepoIdentifier の結果をチェックし、デフォルト値を設定
	repoPath := getRepoIdentifier(gitCloneURL)
	if repoPath == "" {
		repoPath = "リポジトリ" // デフォルト値を設定
	}

	headerText := fmt.Sprintf(
		"🤖 Gemini AI Code Review Result: `%s` ブランチ (%s)",
		featureBranch,
		repoPath,
	)

	headerBlock := slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", headerText, true, false),
	)

	contentSectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", postableText, false, false),
		nil,
		nil,
	)

	// 複数のブロックを配列にまとめる
	blocks := []slack.Block{headerBlock, contentSectionBlock}

	// 3. Webhook用のペイロードを構築
	msg := slack.WebhookMessage{
		Text: "",
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
