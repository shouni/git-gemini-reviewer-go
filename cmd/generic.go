package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"git-gemini-reviewer-go/internal"
	"git-gemini-reviewer-go/internal/config"
)

// localCfg は generic コマンド固有の設定を保持します。
var localCfg config.ReviewConfig

// genericCmd は、レビュー結果を標準出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、Gitリポジトリの差分をAIでレビューし、結果をターミナルに直接出力します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Cobraの context を使用
		ctx := cmd.Context()

		// 1. internal.ReviewParams に変換
		// RunReviewer の引数に必要な情報のみを渡します。
		params := internal.ReviewParams{
			RepoURL:        localCfg.GitCloneURL,
			LocalPath:      localCfg.LocalPath,
			SSHKeyPath:     localCfg.SSHKeyPath,
			BaseBranch:     localCfg.BaseBranch,
			FeatureBranch:  localCfg.FeatureBranch,
			ModelName:      localCfg.GeminiModelName,
			PromptFilePath: localCfg.PromptFilePath,
		}

		// 2. 共通ロジック (internal.RunReviewer) を呼び出す
		reviewResult, err := internal.RunReviewer(ctx, params)
		if err != nil {
			return err
		}

		// 差分がない場合は処理を終了
		if reviewResult == nil {
			return nil
		}

		// 3. 結果を標準出力
		fmt.Println("\n--- Gemini Code Review Result ---")
		fmt.Println(reviewResult.ReviewComment)
		fmt.Println("------------------------------------")

		return nil
	},
}

func init() {
	// フラグのバリデーション（必須チェックなど）は root.go または Cobra の機能に依存
	genericCmd.Flags().StringVar(&localCfg.GitCloneURL, "git-clone-url", "", "The SSH URL of the Git repository to review.")
	genericCmd.Flags().StringVar(&localCfg.BaseBranch, "base-branch", "main", "The base branch for diff comparison (e.g., 'main').")
	genericCmd.Flags().StringVar(&localCfg.FeatureBranch, "feature-branch", "", "The feature branch to review (e.g., 'feature/my-branch').")
	genericCmd.Flags().StringVar(&localCfg.SSHKeyPath, "ssh-key-path", "~/.ssh/id_rsa", "Path to the SSH private key for Git authentication.")
	genericCmd.Flags().StringVar(&localCfg.PromptFilePath, "prompt-file", "review_prompt.md", "Path to the Markdown file containing the review prompt template.")
	genericCmd.Flags().StringVar(&localCfg.LocalPath, "local-path", os.TempDir()+"/git-reviewer-repos/tmp", "Local path to clone the repository.")
	genericCmd.Flags().StringVar(&localCfg.GeminiModelName, "model", "gemini-2.5-flash", "Gemini model name to use for review (e.g., 'gemini-2.5-flash').")

	// 必須フラグのマーク
	genericCmd.MarkFlagRequired("git-clone-url")
	genericCmd.MarkFlagRequired("feature-branch")

	RootCmd.AddCommand(genericCmd)
}
