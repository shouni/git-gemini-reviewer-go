package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"git-gemini-reviewer-go/internal/services"
	"git-gemini-reviewer-go/prompts"

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
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. レビューモードの取得と確認
		currentReviewMode, err := cmd.Flags().GetString("mode")
		if err != nil {
			return fmt.Errorf("review-mode フラグの取得に失敗しました: %w", err)
		}

		// 2. 環境変数の確認
		if slackWebhookURL == "" {
			return fmt.Errorf("--slack-webhook-url フラグまたは SLACK_WEBHOOK_URL 環境変数の設定が必須です")
		}

		var selectedPrompt string
		switch currentReviewMode {
		case "release":
			selectedPrompt = prompts.ReleasePromptTemplate
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			selectedPrompt = prompts.DetailPromptTemplate
			fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
		default:
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", currentReviewMode)
		}

		// 3. 共通ロジックのための設定構造体を作成
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

		// 4. 一時ディレクトリのクリーンアップ (defer でクリーンアップ処理を追加)
		// デフォルトパスかつ一時ディレクトリである場合にのみクリーンアップを予約
		if cfg.LocalPath != "" && strings.HasPrefix(cfg.LocalPath, os.TempDir()) {
			defer func(path string) {
				if err := os.RemoveAll(path); err != nil {
					log.Printf("WARN: failed to clean up local path '%s': %v", path, err)
				}
			}(cfg.LocalPath)
		}

		// 5. 共通ロジックを実行し、結果を取得
		reviewResult, err := services.RunReviewAndGetResult(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		// Diffなしを結果が空文字列であることで判定するロジックは保持
		if reviewResult == "" {
			fmt.Println("ℹ️ Diffが見つからなかったため、レビューをスキップしました。")
			return nil
		}

		// 6. レビュー結果の出力または Slack への投稿
		if noPostSlack {
			fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップ) ---")
			fmt.Println(reviewResult)
			fmt.Println("--------------------------------------------")
			return nil
		}

		// Slack サービスを使用して投稿
		slackService := services.NewSlackClient(slackWebhookURL)

		fmt.Printf("📤 Slack Webhook URL にレビュー結果を投稿します...\n")

		// PostMessage の呼び出し
		err = slackService.PostMessage(cmd.Context(), reviewResult, cfg.FeatureBranch, cfg.GitCloneURL)
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
		os.Getenv("SLACK_WEBHOOK_URL"),
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

	// 共通の必須フラグは root.go でマークされていると仮定し、ここでは再度の MarkFlagRequired は省略
	// slackCmd.MarkFlagRequired("git-clone-url")
	// slackCmd.MarkFlagRequired("feature-branch")
}
