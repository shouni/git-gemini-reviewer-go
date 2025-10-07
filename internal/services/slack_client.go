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
			Timeout: 10 * time.Second, // ネットワークのハングアップを防止
		},
	}
}

// getRepoIdentifier は、GitのクローンURLから 'owner/repo' 形式の識別子を抽出します。
// HTTP(S)およびSSH形式のURLに対応し、抽出に失敗した場合は空文字列を返します。
func getRepoIdentifier(gitCloneURL string) string {
	// git@github.com:owner/repo.git のようなSSH形式のURLを処理
	if strings.HasPrefix(gitCloneURL, "git@") {
		re := regexp.MustCompile(`:([a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+)`)
		matches := re.FindStringSubmatch(gitCloneURL)
		if len(matches) > 1 {
			return strings.TrimSuffix(matches[1], ".git")
		}
	}

	// HTTP, HTTPS, SSH (ssh://) 形式のURLを処理
	parsedURL, err := url.Parse(gitCloneURL)
	if err != nil {
		log.Printf("WARNING: Failed to parse Git clone URL '%s': %v", gitCloneURL, err)
		return ""
	}

	// パスから .git を削除し、/ で分割
	path := strings.TrimSuffix(parsedURL.Path, ".git")
	parts := strings.Split(path, "/")

	// 空の要素を除外
	var cleanParts []string
	for _, p := range parts {
		if p != "" {
			cleanParts = append(cleanParts, p)
		}
	}

	// 最後の2つの要素を 'owner/repo' として結合
	if len(cleanParts) >= 2 {
		return strings.Join(cleanParts[len(cleanParts)-2:], "/")
	}

	log.Printf("WARNING: Could not determine 'owner/repo' from URL path: %s", parsedURL.Path)
	return ""
}

// PostMessage は、AIによるレビュー結果をSlackのBlock Kit形式で投稿します。
func (c *SlackClient) PostMessage(markdownText string, featureBranch string, gitCloneURL string) error {
	repoIdentifier := getRepoIdentifier(gitCloneURL)
	if repoIdentifier == "" {
		repoIdentifier = "不明なリポジトリ" // 識別子が取得できない場合のフォールバック
	}

	// --- 1. Block Kitコンポーネントの構築 ---

	// ヘッダーブロック
	headerBlock := slack.NewHeaderBlock(
		slack.NewTextBlockObject("plain_text", "🤖 Gemini AI Code Review Result", true, false),
	)

	// ブランチ情報とリポジトリへのボタンを配置するセクション
	var branchAccessory *slack.Accessory
	if gitCloneURL != "" {
		branchAccessory = slack.NewAccessory(
			slack.NewButtonBlockElement(
				"view_repository_button", // Action ID
				repoIdentifier,           // Value
				slack.NewTextBlockObject("plain_text", "リポジトリを見る", true, false),
			).WithURL(strings.TrimSuffix(gitCloneURL, ".git")),
		)
	}
	branchSectionBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("✅ `%s` ブランチのレビューが完了しました。", featureBranch), false, false),
		nil,
		branchAccessory,
	)

	// メインのブロックリストを初期化
	blocks := []slack.Block{headerBlock, branchSectionBlock, slack.NewDividerBlock()}

	// --- 2. レビュー本文を動的にブロックへ変換 ---
	const maxSectionLength = 2900 // Slackセクションブロックの文字数上限(3000)へのバッファ
	const maxBlocks = 50          // メッセージが長くなりすぎないようにブロック数も制限 (Slack上限は100)
	const truncationSuffix = "\n\n... (レビューが長すぎるため省略されました)"

	// レビュー本文を水平線(---)で分割し、セクションごとのブロックを生成
	reviewSections := regexp.MustCompile(`\n---\n?`).Split(markdownText, -1)
	headerRegex := regexp.MustCompile(`(?m)^##\s*(.*)$`)

	for _, sectionText := range reviewSections {
		// ブロック数が上限に近い場合、省略メッセージを追加して終了
		if len(blocks) >= maxBlocks-2 {
			log.Println("WARNING: Review has too many sections, truncating message.")
			blocks = append(blocks, slack.NewSectionBlock(
				slack.NewTextBlockObject("mrkdwn", truncationSuffix, false, false), nil, nil))
			break
		}

		if strings.TrimSpace(sectionText) == "" {
			continue
		}

		// Markdownの `## Title` を Slackの `*Title*` (太字) に変換
		processedText := headerRegex.ReplaceAllString(sectionText, "*$1*")

		// セクションごとの文字数制限を超えた場合、そのセクションを切り詰める
		if len(processedText) > maxSectionLength {
			log.Printf("WARNING: A review section is too long (%d chars), truncating.", len(processedText))
			processedText = processedText[:maxSectionLength-len(truncationSuffix)] + truncationSuffix
		}

		blocks = append(blocks, slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", processedText, false, false), nil, nil),
			slack.NewDividerBlock(),
		)
	}
	// 最後の余分なDividerを削除
	if len(blocks) > 0 {
		blocks = blocks[:len(blocks)-1]
	}

	// フッターとしてコンテキストブロックを追加
	footerBlock := slack.NewContextBlock(
		"review-context",
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("リポジトリ: `%s`  |  レビュー時刻: %s",
			repoIdentifier, time.Now().Format("2006-01-02 15:04")), false, false),
	)
	blocks = append(blocks, footerBlock)

	// --- 3. Webhookメッセージの作成と送信 ---
	msg := slack.WebhookMessage{
		Text: fmt.Sprintf("Gemini AI レビュー: %s (%s)", featureBranch, repoIdentifier), // 通知用のフォールバックテキスト
		Blocks: &slack.Blocks{
			BlockSet: blocks,
		},
	}

	jsonPayload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	resp, err := c.httpClient.Post(c.WebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to post to Slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack API returned non-OK status code: %d %s", resp.StatusCode, resp.Status)
	}

	return nil
}
