package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	// 共通ロジックと設定を利用するために internal パッケージ群をインポート
	"git-gemini-reviewer-go/internal"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/services"
)

// BacklogConfig は Backlog 連携のための設定を保持します。
type BacklogConfig struct {
	config.ReviewConfig // ReviewConfig を埋め込み、設定の重複を排除
	IssueID             string
	NoPost              bool
}

var backlogCfg BacklogConfig

// backlogCmd は、レビュー結果を Backlog にコメント投稿するコマンドです。
var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "コードレビューを実行し、その結果をBacklogにコメントとして投稿します。",
	Long:  `このコマンドは、Gitリポジトリの差分をAIでレビューし、結果を指定されたBacklog課題にコメントとして投稿します。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// 1. internal.ReviewParams に変換 (IssueID は RunReviewer の責務外のため除外)
		params := internal.ReviewParams{
			RepoURL:        backlogCfg.GitCloneURL,
			LocalPath:      backlogCfg.LocalPath,
			SSHKeyPath:     backlogCfg.SSHKeyPath,
			BaseBranch:     backlogCfg.BaseBranch,
			FeatureBranch:  backlogCfg.FeatureBranch,
			ModelName:      backlogCfg.GeminiModelName,
			PromptFilePath: backlogCfg.PromptFilePath,
		}

		// 2. 共通ロジック (internal.RunReviewer) を呼び出す
		// Git操作と Gemini レビューのロジックが RunReviewer にカプセル化されました。
		reviewResult, err := internal.RunReviewer(ctx, params)
		if err != nil {
			return err
		}

		if reviewResult == nil { // 差分がない場合
			log.Println("No diff found. Backlog comment skipped.")
			return nil
		}

		// 投稿するコメント本文を構築
		finalComment := fmt.Sprintf("## AIコードレビュー結果 (Model: %s)\n\n%s",
			reviewResult.ModelName,
			reviewResult.ReviewComment,
		)

		// 3. Backlogへの投稿処理
		if backlogCfg.NoPost {
			// NoPost フラグが設定されている場合は標準出力
			fmt.Println("\n--- 📝 Backlog Comment (Skipped Posting) ---")
			fmt.Println(finalComment)
			fmt.Println("-------------------------------------------")
			return nil
		}

		log.Println("--- 3. Backlogコメント投稿を開始 ---")

		// Backlogクライアントの初期化
		backlogClient, err := services.NewBacklogClient(os.Getenv("BACKLOG_SPACE_URL"), os.Getenv("BACKLOG_API_KEY"))
		if err != nil {
			return fmt.Errorf("Backlogクライアントの初期化エラー: %w", err)
		}

		// 投稿の実行
		if err := backlogClient.PostComment(backlogCfg.IssueID, finalComment); err != nil {
			return fmt.Errorf("Backlog課題 %s へのコメント投稿に失敗しました: %w", backlogCfg.IssueID, err)
		}

		log.Printf("Backlog課題 %s へのコメント投稿を完了しました。", backlogCfg.IssueID)

		return nil
	},
}

func init() {
	// フラグの定義を backlogCfg のフィールドに関連付け
	backlogCmd.Flags().StringVar(&backlogCfg.GitCloneURL, "git-clone-url", "", "The SSH URL of the Git repository to review.")
	backlogCmd.Flags().StringVar(&backlogCfg.BaseBranch, "base-branch", "main", "The base branch for diff comparison.")
	backlogCmd.Flags().StringVar(&backlogCfg.FeatureBranch, "feature-branch", "", "The feature branch to review.")
	backlogCmd.Flags().StringVar(&backlogCfg.SSHKeyPath, "ssh-key-path", "~/.ssh/id_rsa", "Path to the SSH private key for Git authentication.")
	backlogCmd.Flags().StringVar(&backlogCfg.PromptFilePath, "prompt-file", "review_prompt.md", "Path to the Markdown file containing the review prompt template.")
	backlogCmd.Flags().StringVar(&backlogCfg.LocalPath, "local-path", os.TempDir()+"/git-reviewer-repos/tmp", "Local path to clone the repository.")
	backlogCmd.Flags().StringVar(&backlogCfg.GeminiModelName, "model", "gemini-2.5-flash", "Gemini model name to use for review.")

	// Backlog 固有のフラグ
	backlogCmd.Flags().StringVar(&backlogCfg.IssueID, "issue-id", "", "The Backlog issue ID to post the comment to (e.g., PROJECT-123).")
	backlogCmd.Flags().BoolVar(&backlogCfg.NoPost, "no-post", false, "If true, skips posting to Backlog and prints to stdout.")
	// 必須フラグのマーク
	backlogCmd.MarkFlagRequired("git-clone-url")
	backlogCmd.MarkFlagRequired("feature-branch")
	backlogCmd.MarkFlagRequired("issue-id") // issue-idもBacklog連携では必須

	RootCmd.AddCommand(backlogCmd)
}
