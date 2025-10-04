// cmd/generic.go

package cmd

import (
	_ "embed"
	// "context" は削除
	"fmt"
	"os" // 👈 'os.TempDir()' を使うために追加
	// "os/exec" は削除
	"strings"

	"git-gemini-reviewer-go/internal/services"
	"github.com/spf13/cobra"
)

// NOTE: ルートコマンドから移設された埋め込みプロンプト
//go:embed prompts/release_review_prompt.md
var releasePrompt string
//go:embed prompts/detail_review_prompt.md
var detailPrompt string

// genericCmd 固有のフラグ変数を定義
var (
	// モデル名を受け取る変数。init() でフラグと紐づけられます。
	geminiModel     string
)

// genericCmd は、レビュー結果を標準出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
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

		// ----------------------------------------------------
		// 2. Git Diff の取得 ( GitClient を使ったリモートリポジトリ比較に置き換え)
		// ----------------------------------------------------

		if gitCloneURL == "" {
			return fmt.Errorf("--git-clone-url フラグは必須です")
		}
		if baseBranch == "" || featureBranch == "" {
			return fmt.Errorf("--base-branch と --feature-branch フラグは必須です")
		}

		fmt.Println("🔍 Gitリポジトリを準備し、差分を取得中...")

		// 2-1. GitClientの初期化
		gitClient := services.NewGitClient(localPath, sshKeyPath)
		gitClient.BaseBranch = baseBranch
		gitClient.InsecureSkipHostKeyCheck = skipHostKeyCheck

		// 2-2. クローン/アップデート
		repo, err := gitClient.CloneOrUpdateWithExec(gitCloneURL, localPath)
		if err != nil {
			return fmt.Errorf("リポジトリのクローン/更新に失敗しました: %w", err)
		}

		// 2-3. フェッチ
		if err := gitClient.Fetch(repo); err != nil {
			return fmt.Errorf("リモートからの最新情報取得 (fetch) に失敗しました: %w", err)
		}

		// 2-4. Diffの取得 (3点比較)
		diffContent, err := gitClient.GetCodeDiff(repo, baseBranch, featureBranch)
		if err != nil {
			return fmt.Errorf("リモートブランチ間のDiff取得に失敗しました: %w", err)
		}

		if strings.TrimSpace(diffContent) == "" {
			fmt.Println("ℹ️ 差分が見つかりませんでした。レビューをスキップします。")
			return nil
		}
		// ----------------------------------------------------


		// 3. Gemini クライアントの初期化
		client, err := services.NewGeminiClient(geminiModel)
		if err != nil {
			return fmt.Errorf("Geminiクライアントの初期化に失敗しました: %w", err)
		}
		defer client.Close()

		// 4. Gemini AIにレビューを依頼
		fmt.Println("🚀 Gemini AIによるコードレビューを開始します...")
		// context.Background() ではなく cmd.Context() を使用
		reviewResult, err := client.ReviewCodeDiff(cmd.Context(), diffContent, selectedPrompt)
		if err != nil {
			return fmt.Errorf("コードレビュー中にエラーが発生しました: %w", err)
		}

		// 5. レビュー結果の出力
		fmt.Println("\n--- Gemini AI レビュー結果 ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------")

		return nil
	},
}

// init 関数は、コマンドを rootCmd に登録し、フラグを定義します。
func init() {
	RootCmd.AddCommand(genericCmd)

	// すべての強力なフラグを定義
	genericCmd.Flags().StringVar(
		&gitCloneURL,
		"git-clone-url",
		"",
		"The SSH URL of the Git repository to review.",
	)
	genericCmd.Flags().StringVar(
		&baseBranch,
		"base-branch",
		"main",
		"The base branch for diff comparison (e.g., 'main').",
	)
	genericCmd.Flags().StringVar(
		&featureBranch,
		"feature-branch",
		"",
		"The feature branch to review (e.g., 'feature/my-branch').",
	)
	genericCmd.Flags().StringVar(
		&sshKeyPath,
		"ssh-key-path",
		"~/.ssh/id_rsa",
		"Path to the SSH private key for Git authentication.",
	)
	genericCmd.Flags().StringVar(
		&localPath,
		"local-path",
		os.TempDir() + "/git-reviewer-repos/tmp-generic", // OSの一時ディレクトリを使用
		"Local path to clone the repository.",
	)
	genericCmd.Flags().BoolVar(
		&skipHostKeyCheck,
		"skip-host-key-check",
		false,
		"If set, skips SSH host key checking (StrictHostKeyChecking=no). Use with caution.",
	)

	// モデルフラグ (既存)
	genericCmd.Flags().StringVar(
		&geminiModel,
		"model",
		"gemini-2.5-flash",
		"Gemini model name to use for review (e.g., 'gemini-2.5-flash').",
	)

	// 必須フラグの設定
	genericCmd.MarkFlagRequired("git-clone-url")
	genericCmd.MarkFlagRequired("feature-branch")
}
