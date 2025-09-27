package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git-gemini-reviewer-go/internal/services"

	"github.com/spf13/cobra"
)

var genericCfg ReviewConfig

func init() {
	RootCmd.AddCommand(genericCmd)

	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// --- フラグの定義 ---
	genericCmd.Flags().StringVar(&genericCfg.GitCloneURL, "git-clone-url", "", "レビュー対象のGitリポジトリURL")
	genericCmd.MarkFlagRequired("git-clone-url")

	genericCmd.Flags().StringVar(&genericCfg.BaseBranch, "base-branch", "", "差分比較の基準ブランチ")
	genericCmd.MarkFlagRequired("base-branch")

	genericCmd.Flags().StringVar(&genericCfg.FeatureBranch, "feature-branch", "", "レビュー対象のフィーチャーブランチ")
	genericCmd.MarkFlagRequired("feature-branch")

	genericCmd.Flags().StringVar(&genericCfg.LocalPath, "local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	genericCmd.Flags().StringVar(&genericCfg.GeminiModelName, "gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")

	genericCmd.Flags().StringVar(&genericCfg.SSHKeyPath, "ssh-key-path", "~/.ssh/id_rsa",
		"SSH認証に使用する秘密鍵ファイルのパス (デフォルト: ~/.ssh/id_rsa)")

	genericCmd.Flags().StringVar(&genericCfg.PromptFilePath, "prompt-file", "review_prompt.md",
		"Geminiへのレビュー依頼に使用するプロンプトファイルのパス")
}

var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "Gitリポジトリの差分をレビューし、結果を標準出力します。",
	Long:  `このモードは、差分レビューの結果を標準出力に出力します。`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// 1. Gitクライアントを初期化し、リポジトリを処理
		gitClient := services.NewGitClient(genericCfg.LocalPath, genericCfg.SSHKeyPath)
		repo, err := gitClient.CloneOrOpen(genericCfg.GitCloneURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error processing repository: %v\n", err)
			os.Exit(1)
		}

		// 1.5. 最新の変更をフェッチ
		if err := gitClient.Fetch(repo); err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching latest changes: %v\n", err)
			os.Exit(1)
		}

		// 2. コード差分を取得
		codeDiff, err := gitClient.GetCodeDiff(repo, genericCfg.BaseBranch, genericCfg.FeatureBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting code diff: %v\n", err)
			os.Exit(1)
		}

		if codeDiff == "" {
			fmt.Println("レビュー対象の差分がありませんでした。処理を終了します。")
			os.Exit(0)
		}

		fmt.Println("--- 差分取得完了。Geminiにレビューを依頼します... ---")

		// 3. Geminiクライアントを初期化
		geminiClient, err := services.NewGeminiClient(genericCfg.GeminiModelName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing Gemini client: %v\n", err)
			os.Exit(1)
		}
		defer geminiClient.Close()

		// 4. Geminiにレビューを依頼 (サービス層の関数を呼び出すだけ)
		reviewResult, err := geminiClient.ReviewCodeDiff(ctx, codeDiff, genericCfg.PromptFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error requesting review from Gemini: %v\n", err)
			os.Exit(1)
		}

		// 5. 結果を標準出力
		fmt.Println("\n--- 📝 Gemini Code Review Result ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------------")
	},
}
