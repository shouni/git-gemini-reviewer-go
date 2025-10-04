package cmd

import (
	_ "embed"
	"fmt"
	"os"

	"git-gemini-reviewer-go/internal/services"
	"github.com/spf13/cobra"
)

//go:embed prompts/release_review_prompt.md
var releasePrompt string
//go:embed prompts/detail_review_prompt.md
var detailPrompt string

// genericCmd は、リモートリポジトリのブランチ比較を Gemini AI に依頼し、結果を標準出力に出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果を標準出力に直接表示します。Backlogなどの外部サービスとの連携は行いません。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. レビューモードの選択
		var selectedPrompt string
		switch reviewMode {
		case "release":
			selectedPrompt = releasePrompt
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			selectedPrompt = detailPrompt
			fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
		default:
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
		}

		// 2. 共通ロジックのための設定構造体を作成
		// root.go で定義されたグローバル変数 (gitCloneURL, baseBranchなど) を使用
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

		// 3. 共通ロジックを実行し、結果を取得
		reviewResult, err := services.RunReviewAndGetResult(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		if reviewResult == "" {
			return nil // Diffなしでスキップされた場合
		}

		// 4. レビュー結果の出力 (generic 固有の処理)
		fmt.Println("\n--- Gemini AI レビュー結果 ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(genericCmd)

	// NOTE: Git関連のフラグ (gitCloneURL, baseBranch, featureBranchなど) および
	// model は root.go の PersistentFlags で定義済みのため、ここでは定義しない。
	// local-path のデフォルト値上書きのみを定義する。
	genericCmd.Flags().StringVar(
		&localPath, // cmd/root.go で定義された変数にバインドし、デフォルト値を上書き
		"local-path",
		os.TempDir()+"/git-reviewer-repos/tmp-generic", // generic 用の専用パス
		"Local path to clone the repository.",
	)

	// genericCmd 固有の必須フラグはないため、ここでは MarkFlagRequired は不要
	// 共通の必須フラグは root.go でマークされている
	genericCmd.MarkFlagRequired("git-clone-url")
	genericCmd.MarkFlagRequired("feature-branch")
}
