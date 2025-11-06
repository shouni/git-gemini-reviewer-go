package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/services"

	"github.com/shouni/go-notifier/pkg/notifier"
	"github.com/spf13/cobra"
)

// --- 構造体: Slack認証情報 ---

// slackAuthInfo は、Slack投稿に必要な認証情報と投稿情報をカプセル化します。
type slackAuthInfo struct {
	WebhookURL string
	Username   string
	IconEmoji  string
	Channel    string
}

// --- コマンド定義 ---

// slackCmd 固有のフラグ変数を定義
var (
	noPostSlack bool // 投稿をスキップする
)

// slackCmd は、レビュー結果を Slack にメッセージとして投稿するコマンドです。
var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "コードレビューを実行し、その結果をSlackの指定されたチャンネルに投稿します。",
	// 【修正 3】行番号 28: RunE: runSlackCommand, のコメントアウトされた行を削除
	RunE: runSlackCommand,
}

func init() {
	slackCmd.Flags().BoolVar(&noPostSlack, "no-post", false, "投稿をスキップし、結果を標準出力する")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runSlackCommand はコマンドの主要な実行ロジックを含みます。
func runSlackCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. Slack 連携に必要な環境変数を取得し、構造体にまとめる
	authInfo := getSlackAuthInfo()

	if authInfo.WebhookURL == "" {
		return fmt.Errorf("SLACK_WEBHOOK_URL 環境変数の設定が必須です。")
	}

	// 2. 共通ロジックを実行し、結果を取得 (ReviewConfig は PersistentPreRunE で構築済み)
	reviewResult, err := services.RunReviewAndGetResult(ctx, ReviewConfig)
	if err != nil {
		// services層で発生したエラーはそのまま返す
		return err
	}

	if reviewResult == "" {
		slog.Info("Diffが見つからなかったため、レビューをスキップしました。")
		return nil
	}

	// 3. no-post フラグによる出力分岐
	if noPostSlack {
		printSlackResult(reviewResult)
		return nil
	}

	// 4. Slack投稿処理を実行
	err = postToSlack(ctx, reviewResult, authInfo)
	if err != nil {
		// 投稿失敗時: エラーログとレビュー結果の出力順序は適切
		printSlackResult(reviewResult) // レビュー結果を標準出力 (fmt.Println)
		slog.Error("Slackへのメッセージ投稿に失敗しました。", "error", err)

		return fmt.Errorf("Slack へのメッセージ投稿に失敗しました。詳細はログを確認してください。")
	}

	slog.Info("レビュー結果を Slack に投稿しました。")
	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// getSlackAuthInfo は、環境変数から Slack 認証情報を取得します。
func getSlackAuthInfo() slackAuthInfo {
	return slackAuthInfo{
		WebhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
		Username:   os.Getenv("SLACK_USERNAME"),
		IconEmoji:  os.Getenv("SLACK_ICON_EMOJI"),
		Channel:    os.Getenv("SLACK_CHANNEL"),
	}
}

// postToSlack は、Slackへの投稿処理の責務を持ちます。
// グローバル変数への依存を減らし、必要な情報を構造体として受け取ります。
func postToSlack(
	ctx context.Context,
	content string,
	authInfo slackAuthInfo,
) error {
	// 1. sharedClient の利用
	if sharedClient == nil {
		// 【修正 8】行番号 92-94: エラーメッセージを簡潔化
		return fmt.Errorf("内部エラー: HTTP クライアントが初期化されていません")
	}

	// 2. slackService の初期化 (sharedClient を利用)
	slackService := notifier.NewSlackNotifier(
		*sharedClient,
		authInfo.WebhookURL,
		authInfo.Username,
		authInfo.IconEmoji,
		authInfo.Channel,
	)

	// slogへ移行
	slog.Info("Slack Webhook URL にレビュー結果を投稿します...", "channel", authInfo.Channel)

	// ヘッダー文字列の作成 (ブランチ情報を結合)
	title := fmt.Sprintf(
		"AIコードレビュー結果 (ブランチ: `%s` ← `%s`)",
		ReviewConfig.BaseBranch,
		ReviewConfig.FeatureBranch,
	)

	// SendTextWithHeader は content を整形し、ヘッダー情報を含めて投稿する
	return slackService.SendTextWithHeader(ctx, title, content)
}

// printSlackResult は noPost 時に結果を標準出力します。
func printSlackResult(result string) {
	// 標準出力 (fmt.Println) は維持
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
