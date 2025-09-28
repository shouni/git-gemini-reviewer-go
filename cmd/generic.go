package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	// 💡 共通ロジックを呼び出すために internal パッケージをインポート
	"git-gemini-reviewer-go/internal"
	// 💡 共通設定構造体を利用するために internal/config パッケージをインポート
	"git-gemini-reviewer-go/internal/config"
)

// localCfg は generic コマンド固有の設定を保持します。
// 💡 config.ReviewConfig を利用することで、設定の重複を排除します。
var localCfg config.ReviewConfig

// genericCmd は、レビュー結果を標準出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "Perform a code review and output the result to stdout.",
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
		// 💡 Git操作と Gemini レビューのロジックが RunReviewer にカプセル化されました。
		reviewResult, err := internal.RunReviewer(ctx, params)
		if err != nil {
			return err
		}

		// 差分がない場合は処理を終了
		if reviewResult == nil {
			return nil
		}

		// 3. 結果を標準出力
		fmt.Println("\n--- 📝 Gemini Code Review Result ---")
		fmt.Println(reviewResult.ReviewComment)
		fmt.Println("------------------------------------")

		return nil
	},
}

func init() {
	// 💡 フラグ定義を localCfg のフィールドに関連付け
	// フラグのバリデーション（必須チェックなど）は root.go または Cobra の機能に依存
	genericCmd.Flags().StringVar(&localCfg.GitCloneURL, "git-clone-url", "", "The SSH URL of the Git repository to review.")
	genericCmd.Flags().StringVar(&localCfg.BaseBranch, "base-branch", "main", "The base branch for diff comparison (e.g., 'main').")
	genericCmd.Flags().StringVar(&localCfg.FeatureBranch, "feature-branch", "", "The feature branch to review (e.g., 'feature/my-branch').")
	genericCmd.Flags().StringVar(&localCfg.SSHKeyPath, "ssh-key-path", "~/.ssh/id_rsa", "Path to the SSH private key for Git authentication.")
	genericCmd.Flags().StringVar(&localCfg.PromptFilePath, "prompt-file", "review_prompt.md", "Path to the Markdown file containing the review prompt template.")
	genericCmd.Flags().StringVar(&localCfg.LocalPath, "local-path", os.TempDir()+"/git-reviewer-repos/tmp", "Local path to clone the repository.")
	genericCmd.Flags().StringVar(&localCfg.GeminiModelName, "model", "gemini-2.5-flash", "Gemini model name to use for review (e.g., 'gemini-2.5-flash').")

	RootCmd.AddCommand(genericCmd)
}
