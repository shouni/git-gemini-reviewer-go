package cmd

import (
	"context"
	"fmt"
	"git-gemini-reviewer-go/internal/services"
	"log"
	"os"

	"git-gemini-reviewer-go/internal/config"

	"github.com/shouni/go-notifier/pkg/notifier"
	"github.com/spf13/cobra"
)

// slackCmd 固有のフラグ変数を定義
var (
	noPostSlack bool // noPostSlack のみを固有フラグとして残す
)

// slackCmd は、レビュー結果を Slack にメッセージとして投稿するコマンドです。
var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "コードレビューを実行し、その結果をSlackの指定されたチャンネルに投稿します。",
	// ロジックを外部関数に分離
	RunE: runSlackCommand,
}

func init() {
	// RootCmd.AddCommand(slackCmd) // clibase.Execute の引数で処理されることを前提

	// Slack 固有の no-post フラグのみを定義
	slackCmd.Flags().BoolVar(&noPostSlack, "no-post", false, "投稿をスキップし、結果を標準出力する")

	// NOTE: slackWebhookURL の設定は、もしフラグとして提供したい場合、
	// AppFlags に追加し、addAppPersistentFlags で定義するか、
	// あるいは環境変数 SLACK_WEBHOOK_URL の利用に限定します。
	// このコードでは環境変数 SLACK_WEBHOOK_URL の利用に限定されていると解釈します。
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runSlackCommand はコマンドの主要な実行ロジックを含みます。
func runSlackCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Slack 連携に必要な環境変数を取得
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	slackUsername := os.Getenv("SLACK_USERNAME")
	slackIconEmoji := os.Getenv("SLACK_ICON_EMOJI")
	slackChannel := os.Getenv("SLACK_CHANNEL")

	if webhookURL == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL 環境変数の設定が必須です")
	}

	// 2. 共通設定の作成 (Flags (AppFlags) を利用)
	params := CreateReviewConfigParams{
		ReviewMode:       Flags.ReviewMode,
		GeminiModel:      Flags.GeminiModel,
		GitCloneURL:      Flags.GitCloneURL,
		BaseBranch:       Flags.BaseBranch,
		FeatureBranch:    Flags.FeatureBranch,
		SSHKeyPath:       Flags.SSHKeyPath,
		LocalPath:        Flags.LocalPath,
		SkipHostKeyCheck: Flags.SkipHostKeyCheck,
	}
	// NOTE: CreateReviewConfig は他の場所で定義されていると仮定
	cfg, err := CreateReviewConfig(params)
	if err != nil {
		return err
	}

	// 3. 共通ロジックを実行し、結果を取得
	reviewResult, err := services.RunReviewAndGetResult(ctx, cfg)
	if err != nil {
		return err
	}

	if reviewResult == "" {
		log.Println("✅ Diffが見つからなかったため、レビューをスキップしました。")
		return nil
	}

	// 4. no-post フラグによる出力分岐
	if noPostSlack {
		printSlackResult(reviewResult)
		return nil
	}

	// 5. Slack投稿処理を実行
	err = postToSlack(ctx, webhookURL, reviewResult, cfg, slackUsername, slackIconEmoji, slackChannel)
	if err != nil {
		log.Printf("ERROR: Slack へのメッセージ投稿に失敗しました: %v\n", err)
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

// postToSlack は、Slackへの投稿処理の責務を持ちます。
// sharedClient を利用するように修正し、Slack固有の情報を引数として受け取ります。
func postToSlack(
	ctx context.Context,
	webhookURL,
	content string,
	cfg config.ReviewConfig,
	username,
	iconEmoji,
	channel string,
) error {
	// 1. sharedClient の利用
	if sharedClient == nil {
		return fmt.Errorf("内部エラー: HTTP クライアント (sharedClient) が初期化されていません")
	}

	// 2. SlackNotifier の初期化 (sharedClient を利用)
	slackNotifier := notifier.NewSlackNotifier(
		*sharedClient,
		webhookURL,
		username,
		iconEmoji,
		channel,
	)

	fmt.Printf("📤 Slack Webhook URL にレビュー結果を投稿します...\n")

	// ヘッダー文字列の作成 (ブランチ情報を結合)
	title := fmt.Sprintf(
		"📝 AIコードレビュー結果 (ブランチ: `%s` ← `%s`)",
		cfg.BaseBranch,
		cfg.FeatureBranch,
	)

	// SendTextWithHeader は content を整形し、ヘッダー情報を含めて投稿する
	return slackNotifier.SendTextWithHeader(ctx, title, content)
}

// printSlackResult は noPost 時に結果を標準出力します。
func printSlackResult(result string) {
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
