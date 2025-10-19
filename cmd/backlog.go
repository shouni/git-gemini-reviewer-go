package cmd

import (
	"fmt"
	"log"
	"os"

	"git-gemini-reviewer-go/internal/services"
	"git-gemini-reviewer-go/prompts"

	"github.com/spf13/cobra"
)

// backlogCmd 固有のフラグ変数のみを定義
var (
	issueID string
	noPost  bool
)

// backlogCmd は、レビュー結果を Backlog にコメントとして投稿するコマンドです。
var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "コードレビューを実行し、その結果をBacklogにコメントとして投稿します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をBacklogの指定された課題にコメントとして自動で投稿します。これにより、手動でのレビュー結果転記の手間を省きます。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 環境変数の確認
		backlogAPIKey := os.Getenv("BACKLOG_API_KEY")
		backlogSpaceURL := os.Getenv("BACKLOG_SPACE_URL")

		if backlogAPIKey == "" || backlogSpaceURL == "" {
			return fmt.Errorf("Backlog連携には環境変数 BACKLOG_API_KEY および BACKLOG_SPACE_URL が必須です")
		}

		// 2. レビューモードに基づいたプロンプトの選択
		var selectedPrompt string
		// reviewMode は cmd/root.go の Persistent Flag の変数を使用
		switch reviewMode {
		case "release":
			// 変更点: services.ReleasePromptTemplate を使用
			selectedPrompt = prompts.ReleasePromptTemplate
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			// 変更点: services.DetailPromptTemplate を使用
			selectedPrompt = prompts.DetailPromptTemplate
			fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
		default:
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
		}

		// 3. 共通ロジックのための設定構造体を作成
		// すべて cmd/root.go で定義された共通変数を使用
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

		// 5. レビュー結果の出力または Backlog への投稿 (Backlog固有の処理)
		if noPost {
			fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップ) ---")
			fmt.Println(reviewResult)
			fmt.Println("--------------------------------------------")
			return nil
		}

		if issueID == "" {
			return fmt.Errorf("--issue-id フラグが指定されていません。Backlogに投稿するには必須です。")
		}

		// Backlog サービスを使用して投稿
		backlogService, err := services.NewBacklogClient(backlogSpaceURL, backlogAPIKey)
		if err != nil {
			return fmt.Errorf("Backlogクライアントの初期化に失敗しました: %w", err)
		}

		fmt.Printf("📤 Backlog 課題 ID: %s にレビュー結果を投稿します...\n", issueID)

		err = backlogService.PostComment(cmd.Context(), issueID, reviewResult)
		if err != nil {
			log.Printf("ERROR: Backlog へのコメント投稿に失敗しました (課題ID: %s): %v\n", issueID, err)
			fmt.Println("\n--- Gemini AI レビュー結果 (Backlog投稿失敗) ---")
			fmt.Println(reviewResult)
			fmt.Println("----------------------------------------")
			return fmt.Errorf("Backlog課題 %s へのコメント投稿に失敗しました。詳細は上記レビュー結果を確認してください。", issueID)
		}

		fmt.Printf("✅ レビュー結果を Backlog 課題 ID: %s に投稿しました。\n", issueID)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// Backlog 固有のフラグのみをここで定義する
	backlogCmd.Flags().StringVar(&issueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
	backlogCmd.Flags().BoolVar(&noPost, "no-post", false, "投稿をスキップし、結果を標準出力する")

	// local-path のデフォルト値上書き
	// localPath は cmd/root.go で定義された変数にバインドし、デフォルト値を上書き
	backlogCmd.Flags().StringVar(
		&localPath,
		"local-path",
		os.TempDir()+"/git-reviewer-repos/tmp-backlog",
		"Local path to clone the repository.",
	)

	// 必須フラグの設定
	backlogCmd.MarkFlagRequired("git-clone-url")
	backlogCmd.MarkFlagRequired("feature-branch")
}
