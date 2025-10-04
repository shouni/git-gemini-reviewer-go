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
			log.Printf("⚠️ Backlog への投稿に失敗しました: %v\n", err)

			fmt.Println("\n--- Gemini AI レビュー結果 (投稿失敗) ---")
			fmt.Println(reviewResult)
			fmt.Println("----------------------------------------")
			return fmt.Errorf("Backlog への投稿に失敗しましたが、レビュー結果は上記に出力されています。")
		}

		fmt.Printf("✅ レビュー結果を Backlog 課題 ID: %s に投稿しました。\n", issueID)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// Backlog 固有のフラグ
	backlogCmd.Flags().StringVar(&issueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
	backlogCmd.Flags().BoolVar(&noPost, "no-post", false, "投稿をスキップし、結果を標準出力する")

	// Git連携フラグ (genericCmd から移植)
	backlogCmd.Flags().StringVar(
		&gitCloneURL,
		"git-clone-url",
		"",
		"The SSH URL of the Git repository to review.",
	)
	backlogCmd.Flags().StringVar(
		&baseBranch,
		"base-branch",
		"main",
		"The base branch for diff comparison (e.g., 'main').",
	)
	backlogCmd.Flags().StringVar(
		&featureBranch,
		"feature-branch",
		"",
		"The feature branch to review (e.g., 'feature/my-branch').",
	)
	backlogCmd.Flags().StringVar(
		&sshKeyPath,
		"ssh-key-path",
		"~/.ssh/id_rsa",
		"Path to the SSH private key for Git authentication.",
	)
	backlogCmd.Flags().StringVar(
		&localPath,
		"local-path",
		os.TempDir() + "/git-reviewer-repos/tmp-backlog", // Backlog用に別のパスを使用
		"Local path to clone the repository.",
	)
	backlogCmd.Flags().BoolVar(
		&skipHostKeyCheck,
		"skip-host-key-check",
		false,
		"If set, skips SSH host key checking (StrictHostKeyChecking=no). Use with caution.",
	)

	// モデルフラグ (既存)
	backlogCmd.Flags().StringVar(
		&backlogGeminiModel,
		"model",
		"gemini-2.5-flash",
		"Gemini model name to use for review (e.g., 'gemini-2.5-flash').",
	)

	// 必須フラグの設定（Backlog連携には issue-id 以外に Git連携フラグも必須に）
	backlogCmd.MarkFlagRequired("git-clone-url")
	backlogCmd.MarkFlagRequired("feature-branch")
	// issue-id は --no-post の場合は不要なので、あえて MarkFlagRequired にしません
}
