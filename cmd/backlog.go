package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git-gemini-reviewer-go/internal/services" // サービス層をインポート

	"github.com/spf13/cobra"
)

// backlogCfg は Backlog モードの引数を保持します。
var backlogCfg struct {
	ReviewConfig
	NoPost bool // Backlogモード固有のフラグ
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// LocalPath のデフォルト値を設定
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// --- フラグの定義 ---

	// 必須引数
	backlogCmd.Flags().StringVar(&backlogCfg.GitCloneURL, "git-clone-url", "", "レビュー対象のGitリポジトリURL")
	backlogCmd.MarkFlagRequired("git-clone-url")

	backlogCmd.Flags().StringVar(&backlogCfg.BaseBranch, "base-branch", "", "差分比較の基準ブランチ")
	backlogCmd.MarkFlagRequired("base-branch")

	backlogCmd.Flags().StringVar(&backlogCfg.FeatureBranch, "feature-branch", "", "レビュー対象のフィーチャーブランチ")
	backlogCmd.MarkFlagRequired("feature-branch")

	// 任意の引数
	backlogCmd.Flags().StringVar(&backlogCfg.LocalPath, "local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	backlogCmd.Flags().StringVar(&backlogCfg.IssueID, "issue-id", "",
		"関連する課題ID (Backlog投稿時には必須/スキップ時は任意)")

	backlogCmd.Flags().StringVar(&backlogCfg.GeminiModelName, "gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")

	backlogCmd.Flags().StringVar(&backlogCfg.SSHKeyPath, "ssh-key-path", "",
		"SSH認証に使用する秘密鍵ファイルのパス (例: ~/.ssh/id_rsa)")

	backlogCmd.Flags().StringVar(&backlogCfg.PromptFilePath, "prompt-file", "review_prompt.md",
		"Geminiへのレビュー依頼に使用するプロンプトファイルのパス")

	// Backlogモード固有のフラグ
	backlogCmd.Flags().BoolVar(&backlogCfg.NoPost, "no-post", false,
		"レビュー結果をBacklogにコメント投稿するのをスキップします。")
}

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Gitリポジトリの差分をレビューし、Backlogにコメント投稿します。",
	Long:  `このモードは、差分レビューの結果をBacklogの課題にコメントとして投稿します。`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// --- Git/Geminiレビュー実行ロジック ---

		// 1. Gitクライアントを初期化し、リポジトリを処理
		gitClient := services.NewGitClient(backlogCfg.LocalPath, backlogCfg.SSHKeyPath)
		repo, err := gitClient.CloneOrOpen(backlogCfg.GitCloneURL)
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
		codeDiff, err := gitClient.GetCodeDiff(repo, backlogCfg.BaseBranch, backlogCfg.FeatureBranch)
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
		geminiClient, err := services.NewGeminiClient(backlogCfg.GeminiModelName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing Gemini client: %v\n", err)
			os.Exit(1)
		}
		defer geminiClient.Close()

		// 4. Geminiにレビューを依頼
		reviewResult, err := geminiClient.ReviewCodeDiff(ctx, codeDiff, backlogCfg.PromptFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error requesting review from Gemini: %v\n", err)
			os.Exit(1)
		}

		// --- Backlog投稿ロジック ---

		// 5. Backlog投稿の条件チェック
		if backlogCfg.NoPost {
			fmt.Println("--- ⚠️ --no-post が指定されました。Backlogへのコメント投稿はスキップし、結果は標準出力されます。 ---")
			fmt.Println("\n--- 📝 Gemini Code Review Result ---")
			fmt.Println(reviewResult)
			fmt.Println("------------------------------------")
			return // 投稿せずに終了
		}

		// 投稿する場合の IssueID 必須チェック
		if backlogCfg.IssueID == "" {
			fmt.Fprintln(os.Stderr, "エラー: Backlogへコメント投稿するには --issue-id が必須です。投稿をスキップする場合は --no-post を指定してください。")
			os.Exit(1)
		}

		// 6. Backlogクライアントを初期化
		backlogClient, err := services.NewBacklogClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing Backlog client: %v\n", err)
			os.Exit(1)
		}

		// 7. コメントを投稿
		if err := backlogClient.PostComment(backlogCfg.IssueID, reviewResult); err != nil {
			fmt.Fprintf(os.Stderr, "Error posting comment to Backlog: %v\n", err)
			os.Exit(1)
		}
	},
}
