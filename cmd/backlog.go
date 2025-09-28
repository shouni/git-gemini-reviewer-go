package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git-gemini-reviewer-go/internal/services" // サービス層をインポート

	"github.com/spf13/cobra"
)

// BacklogConfig は Backlog モードの引数を保持するローカル用の構造体です。
// ReviewConfig と Backlog固有のフラグを結合します。
type BacklogConfig struct {
	// ReviewConfig のフィールドをすべて含める（簡略化のため、ここでは手動で定義）
	GitCloneURL     string
	BaseBranch      string
	FeatureBranch   string
	LocalPath       string
	IssueID         string
	GeminiModelName string
	SSHKeyPath      string
	PromptFilePath  string

	// Backlogモード固有のフラグ
	NoPost bool
}

// 🚨 グローバル変数 backlogCfg は削除し、RunE内でローカルな BacklogConfig を使用します。
// var backlogCfg struct { ... }

func init() {
	RootCmd.AddCommand(backlogCmd)

	// LocalPath のデフォルト値を設定
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// --- フラグの定義 (RunE内で値を取得できるよう、バインドはせずに定義のみを行う) ---

	// 必須引数
	backlogCmd.Flags().String("git-clone-url", "", "レビュー対象のGitリポジトリURL")
	backlogCmd.Flags().String("base-branch", "", "差分比較の基準ブランチ")
	backlogCmd.Flags().String("feature-branch", "", "レビュー対象のフィーチャーブランチ")

	backlogCmd.MarkFlagRequired("git-clone-url")
	backlogCmd.MarkFlagRequired("base-branch")
	backlogCmd.MarkFlagRequired("feature-branch")

	// 任意の引数
	backlogCmd.Flags().String("local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	backlogCmd.Flags().String("issue-id", "",
		"関連する課題ID (Backlog投稿時には必須/スキップ時は任意)")

	backlogCmd.Flags().String("gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")

	// SSHキーパスのデフォルト値はここで空にしておき、サービス側で適切なデフォルトを扱うか、
	// ユーザーに明示的に指定させるのが望ましい。
	backlogCmd.Flags().String("ssh-key-path", "~/.ssh/id_rsa",
		"SSH認証に使用する秘密鍵ファイルのパス (デフォルト: ~/.ssh/id_rsa)")

	backlogCmd.Flags().String("prompt-file", "review_prompt.md",
		"Geminiへのレビュー依頼に使用するプロンプトファイルのパス")

	// Backlogモード固有のフラグ
	backlogCmd.Flags().Bool("no-post", false,
		"レビュー結果をBacklogにコメント投稿するのをスキップします。")
}

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Gitリポジトリの差分をレビューし、Backlogにコメント投稿します。",
	Long:  `このモードは、差分レビューの結果をBacklogの課題にコメントとして投稿します。`,
	// RunE を使用し、エラーを返して Cobra のエラーハンドリングに任せます。
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// 1. ローカルな BacklogConfig インスタンスを作成し、フラグの値を読み込む
		var cfg BacklogConfig
		var err error

		// フラグの値を取得 (エラーチェックは Get* の中で行われますが、ここでは nil チェックのために変数に格納)
		cfg.GitCloneURL, err = cmd.Flags().GetString("git-clone-url")
		if err != nil {
			return err
		}
		cfg.BaseBranch, err = cmd.Flags().GetString("base-branch")
		if err != nil {
			return err
		}
		cfg.FeatureBranch, err = cmd.Flags().GetString("feature-branch")
		if err != nil {
			return err
		}
		cfg.LocalPath, err = cmd.Flags().GetString("local-path")
		if err != nil {
			return err
		}
		cfg.IssueID, err = cmd.Flags().GetString("issue-id")
		if err != nil {
			return err
		}
		cfg.GeminiModelName, err = cmd.Flags().GetString("gemini-model-name")
		if err != nil {
			return err
		}
		cfg.SSHKeyPath, err = cmd.Flags().GetString("ssh-key-path")
		if err != nil {
			return err
		}
		cfg.PromptFilePath, err = cmd.Flags().GetString("prompt-file")
		if err != nil {
			return err
		}
		cfg.NoPost, err = cmd.Flags().GetBool("no-post")
		if err != nil {
			return err
		}

		// --- Git/Geminiレビュー実行ロジック ---

		// 2. Gitクライアントを初期化し、リポジトリを処理
		gitClient := services.NewGitClient(cfg.LocalPath, cfg.SSHKeyPath)
		repo, err := gitClient.CloneOrOpen(cfg.GitCloneURL)
		if err != nil {
			return fmt.Errorf("error processing repository: %w", err)
		}

		// 2.5. 最新の変更をフェッチ
		if err := gitClient.Fetch(repo); err != nil {
			return fmt.Errorf("error fetching latest changes: %w", err)
		}

		// 3. コード差分を取得
		codeDiff, err := gitClient.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch)
		if err != nil {
			return fmt.Errorf("error getting code diff: %w", err)
		}

		if codeDiff == "" {
			fmt.Println("レビュー対象の差分がありませんでした。処理を終了します。")
			return nil // 差分がない場合は成功として終了
		}

		fmt.Println("--- 差分取得完了。Geminiにレビューを依頼します... ---")

		// 4. Geminiクライアントを初期化
		geminiClient, err := services.NewGeminiClient(cfg.GeminiModelName)
		if err != nil {
			return fmt.Errorf("error initializing Gemini client: %w", err)
		}
		defer geminiClient.Close()

		// 5. Geminiにレビューを依頼
		reviewResult, err := geminiClient.ReviewCodeDiff(ctx, codeDiff, cfg.PromptFilePath)
		if err != nil {
			return fmt.Errorf("error requesting review from Gemini: %w", err)
		}

		// --- Backlog投稿ロジック ---

		// 6. Backlog投稿の条件チェック
		if cfg.NoPost {
			fmt.Println("--- ⚠️ --no-post が指定されました。Backlogへのコメント投稿はスキップし、結果は標準出力されます。 ---")
			fmt.Println("\n--- 📝 Gemini Code Review Result ---")
			fmt.Println(reviewResult)
			fmt.Println("------------------------------------")
			return nil // 投稿せずに成功終了
		}

		// 投稿する場合の IssueID 必須チェック
		if cfg.IssueID == "" {
			return fmt.Errorf("Backlogへコメント投稿するには --issue-id が必須です。投稿をスキップする場合は --no-post を指定してください。")
		}

		// 7. Backlogクライアントを初期化
		backlogClient, err := services.NewBacklogClient()
		if err != nil {
			return fmt.Errorf("error initializing Backlog client: %w", err)
		}

		// 8. コメントを投稿
		if err := backlogClient.PostComment(cfg.IssueID, reviewResult); err != nil {
			return fmt.Errorf("error posting comment to Backlog: %w", err)
		}

		fmt.Printf("✅ Backlog課題ID %s にレビュー結果のコメントを投稿しました。\n", cfg.IssueID)

		return nil
	},
}
