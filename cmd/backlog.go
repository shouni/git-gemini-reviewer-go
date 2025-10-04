package cmd

import (
	_ "embed"
	"fmt"
	"log"
	"os"

	"git-gemini-reviewer-go/internal/services" // GitClient と Backlogサービスのため
	"github.com/spf13/cobra"
)

// NOTE: generic.go と同じプロンプトを埋め込みます。
//go:embed prompts/release_review_prompt.md
var backlogReleasePrompt string
//go:embed prompts/detail_review_prompt.md
var backlogDetailPrompt string

// backlogCmd 固有のフラグ変数を定義
var (
	// Backlog連携に必要なフラグ
	issueID    string
	noPost     bool

	// Git/Gemini 連携に必要なフラグ
	backlogGeminiModel string
	gitCloneURL        string
	baseBranch         string
	featureBranch      string
	sshKeyPath         string
	localPath          string
	skipHostKeyCheck   bool
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
		switch reviewMode {
		case "release":
			selectedPrompt = backlogReleasePrompt
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			selectedPrompt = backlogDetailPrompt
			fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
		default:
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
		}

		// 3. 共通ロジックのための設定構造体を作成
		cfg := services.ReviewConfig{
			GeminiModel:     backlogGeminiModel,
			PromptContent:   selectedPrompt,
			GitCloneURL:     gitCloneURL,
			BaseBranch:      baseBranch,
			FeatureBranch:   featureBranch,
			SSHKeyPath:      sshKeyPath,
			LocalPath:       localPath,
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

		err = backlogService.PostComment(issueID, reviewResult)
		if err != nil {
			// 投稿失敗時に詳細なログを出力し、ユーザーには簡潔なメッセージとレビュー結果のみを表示
			log.Printf("ERROR: Backlog へのコメント投稿に失敗しました (課題ID: %s): %v\n", issueID, err)
			fmt.Println("\n--- Gemini AI レビュー結果 (Backlog投稿失敗) ---")
			fmt.Println(reviewResult)
			fmt.Println("----------------------------------------")
			// ユーザーに表示されるエラーは簡潔に
			return fmt.Errorf("Backlog課題 %s へのコメント投稿に失敗しました。詳細は上記レビュー結果を確認してください。", issueID)
		}

		fmt.Printf("✅ レビュー結果を Backlog 課題 ID: %s に投稿しました。\n", issueID)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// cmd/backlog.go の init 関数全体

	func init() {
		RootCmd.AddCommand(backlogCmd)

		// Backlog 固有のフラグのみをここで定義する
		backlogCmd.Flags().StringVar(&issueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
		backlogCmd.Flags().BoolVar(&noPost, "no-post", false, "投稿をスキップし、結果を標準出力する")

		// 共通フラグは root.go の PersistentFlags を利用するため、ここで再定義しない。
		// ただし、local-path のようにサブコマンド固有のデフォルト値を設定したい場合は、
		// RootCmdで定義された変数にバインドし直すことで上書きできる。
		backlogCmd.Flags().StringVar(
			&localPath, // cmd/root.go で定義された変数にバインドし、デフォルト値を上書き
			"local-path",
			os.TempDir()+"/git-reviewer-repos/tmp-backlog",
			"Local path to clone the repository.",
		)

		// 必須フラグの設定
		// MarkFlagRequired は RootCmdの変数を参照するが、このコマンドで必須であることを明示
		backlogCmd.MarkFlagRequired("git-clone-url")
		backlogCmd.MarkFlagRequired("feature-branch")
	}
}
