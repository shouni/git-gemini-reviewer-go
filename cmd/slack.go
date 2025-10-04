package cmd

package cmd

import (
_ "embed"
"fmt"
"log"
"os"

"git-gemini-reviewer-go/internal/services"
"github.com/spf13/cobra"
)

//go:embed prompts/release_review_prompt.md
var slackReleasePrompt string
//go:embed prompts/detail_review_prompt.md
var slackDetailPrompt string

// slackCmd 固有のフラグ変数を定義
var (
	slackWebhookURL string
	noPostSlack     bool
)

// slackCmd は、レビュー結果を Slack にメッセージとして投稿するコマンドです。
var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "コードレビューを実行し、その結果をSlackの指定されたチャンネルに投稿します。",
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 環境変数の確認
		if slackWebhookURL == "" {
			return fmt.Errorf("--slack-webhook-url フラグまたは SLACK_WEBHOOK_URL 環境変数の設定が必須です")
		}

		// 2. レビューモードに基づいたプロンプトの選択
		var selectedPrompt string
		switch reviewMode {
		case "release":
			selectedPrompt = slackReleasePrompt
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			selectedPrompt = slackDetailPrompt
			fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
		default:
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
		}

		// 3. 共通ロジックのための設定構造体を作成 (cmd/root.go の共通変数を使用)
		cfg := services.ReviewConfig{
			GeminiModel:      geminiModel,
			PromptContent:    selectedPrompt,
			GitCloneURL:      gitCloneURL,
			BaseBranch:       baseBranch,
			FeatureBranch:    featureBranch,
			SSHKeyPath:       sshKeyPath,
			LocalPath:        localPath,
			SkipHostKeyCheck: skipHostKeyCheck,
		}

		// 4. 共通ロジックを実行し、結果を取得
		reviewResult, err := services.RunReviewAndGetResult(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		if reviewResult == "" {
			return nil // Diffなしでスキップされた場合
		}

		// 5. レビュー結果の出力または Slack への投稿
		if noPostSlack {
			fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップ) ---")
			fmt.Println(reviewResult)
			fmt.Println("--------------------------------------------")
			return nil
		}

		// Slack サービスを使用して投稿
		slackService := services.NewSlackClient(slackWebhookURL)

		fmt.Printf("📤 Slack Webhook URL にレビュー結果を投稿します...\n")

		// NOTE: ここでは channelID は WebhookURL に含まれるため、空文字列を渡します。
		err = slackService.PostMessage("", reviewResult)
		if err != nil {
			log.Printf("ERROR: Slack へのコメント投稿に失敗しました: %v\n", err)
			return fmt.Errorf("Slack へのメッセージ投稿に失敗しました。詳細はログを確認してください。")
		}

		fmt.Printf("✅ レビュー結果を Slack に投稿しました。\n")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(slackCmd)

	// Slack 固有のフラグ
	slackCmd.Flags().StringVar(
		&slackWebhookURL,
		"slack-webhook-url",
		os.Getenv("SLACK_WEBHOOK_URL"), // 環境変数からのデフォルト値設定
		"レビュー結果を投稿する Slack Webhook URL。",
	)
	slackCmd.Flags().BoolVar(&noPostSlack, "no-post", false, "投稿をスキップし、結果を標準出力する")

	// local-path のデフォルト値上書き (サブコマンド固有のパス)
	slackCmd.Flags().StringVar(
		&localPath,
		"local-path",
		os.TempDir()+"/git-reviewer-repos/tmp-slack",
		"Local path to clone the repository.",
	)

	// 必須フラグの設定
	slackCmd.MarkFlagRequired("git-clone-url")
	slackCmd.MarkFlagRequired("feature-branch")
}

