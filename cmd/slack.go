package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"git-gemini-reviewer-go/internal/services"

	"git-gemini-reviewer-go/internal/config"
	"github.com/spf13/cobra"
)

// slackCmd 固有のフラグ変数を定義
var (
	slackWebhookURL string
	noPostSlack     bool
)

// slackCmd は、レビュー結果を Slack にメッセージとして投稿するコマンドです。
var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "コードレビューを実行し、その結果をSlackの指定されたチャンネルに投稿します。",
	// ロジックを外部関数に分離
	RunE: runSlackCommand,
}

func init() {
	RootCmd.AddCommand(slackCmd)

	// Slack 固有のフラグ
	slackCmd.Flags().StringVar(
		&slackWebhookURL,
		"slack-webhook-url",
		os.Getenv("SLACK_WEBHOOK_URL"),
		"レビュー結果を投稿する Slack Webhook URL。",
	)
	slackCmd.Flags().BoolVar(&noPostSlack, "no-post", false, "投稿をスキップし、結果を標準出力する")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runSlackCommand はコマンドの主要な実行ロジックを含みます。
func runSlackCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. 環境変数の確認
	if slackWebhookURL == "" {
		return fmt.Errorf("--slack-webhook-url フラグまたは SLACK_WEBHOOK_URL 環境変数の設定が必須です")
	}

	// 2. 共通設定の作成 (グローバル変数への依存をここで解決)
	params := CreateReviewConfigParams{
		ReviewMode:       reviewMode,
		GeminiModel:      geminiModel,
		GitCloneURL:      gitCloneURL,
		BaseBranch:       baseBranch,
		FeatureBranch:    featureBranch,
		SSHKeyPath:       sshKeyPath,
		LocalPath:        localPath,
		SkipHostKeyCheck: skipHostKeyCheck,
	}
	cfg, err := CreateReviewConfig(params)
	if err != nil {
		return err
	}

	// 3. 一時ディレクトリのクリーンアップの予約
	defer setupCleanup(cfg.LocalPath)

	// 4. 共通ロジックを実行し、結果を取得
	reviewResult, err := services.RunReviewAndGetResult(ctx, cfg)
	if err != nil {
		return err
	}

	if reviewResult == "" {
		fmt.Println("ℹ️ Diffが見つからなかったため、レビューをスキップしました。")
		return nil
	}

	// 5. no-post フラグによる出力分岐
	if noPostSlack {
		printSlackResult(reviewResult)
		return nil
	}

	// 6. Slack投稿処理を実行
	err = postToSlack(ctx, slackWebhookURL, reviewResult, cfg)
	if err != nil {
		log.Printf("ERROR: Slack へのコメント投稿に失敗しました: %v\n", err)
		// 投稿失敗時も結果をコンソールに出力
		printSlackResult(reviewResult)
		return fmt.Errorf("Slack へのメッセージ投稿に失敗しました。詳細はログを確認してください。")
	}

	fmt.Printf("✅ レビュー結果を Slack に投稿しました。\n")
	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// setupCleanup は、一時ディレクトリである場合にのみクリーンアップを予約します。
func setupCleanup(path string) {
	// デフォルトパスかつ一時ディレクトリである場合にのみクリーンアップを予約
	if path != "" && strings.HasPrefix(path, os.TempDir()) {
		if err := os.RemoveAll(path); err != nil {
			log.Printf("WARN: failed to clean up local path '%s': %v", path, err)
		}
	}
}

// postToSlack は、Slackへの投稿処理の責務を持ちます。
func postToSlack(ctx context.Context, webhookURL, content string, cfg config.ReviewConfig) error {
	slackService := services.NewSlackClient(webhookURL)
	fmt.Printf("📤 Slack Webhook URL にレビュー結果を投稿します...\n")

	// PostMessage の呼び出し
	// PostMessage のシグネチャを調整し、ブランチ情報を渡せるようにしていると仮定
	return slackService.PostMessage(ctx, content, cfg.FeatureBranch, cfg.GitCloneURL)
}

// printSlackResult は noPost 時に結果を標準出力します。
func printSlackResult(result string) {
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
