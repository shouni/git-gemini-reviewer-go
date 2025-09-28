package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"git-gemini-reviewer-go/internal/services"

	"github.com/spf13/cobra"
)

// ReviewConfig はコマンドライン引数を保持する構造体です。
// グローバル変数ではなく、Run関数内でローカルに利用します。
// （※このファイルには定義がありませんが、他のファイルからインポートされていると仮定します）
// type ReviewConfig struct { ... }

// 🚨 グローバル変数 genericCfg を削除します。
// var genericCfg ReviewConfig

// initCmdFlags は genericCmd のフラグを設定し、設定値を *ReviewConfig にバインドします。
// 💡 CobraのRun関数内でローカル変数にバインドするために、このヘルパー関数を定義します。
func initCmdFlags(cmd *cobra.Command, cfg *ReviewConfig) {
	// defaultLocalPath はローカルパスのデフォルト値を定義
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// --- フラグの定義とバインド ---
	// Cobraはポインタを渡すため、この関数実行後、cfgには値がセットされます。
	cmd.Flags().StringVar(&cfg.GitCloneURL, "git-clone-url", "", "レビュー対象のGitリポジトリURL")
	cmd.Flags().StringVar(&cfg.BaseBranch, "base-branch", "", "差分比較の基準ブランチ")
	cmd.Flags().StringVar(&cfg.FeatureBranch, "feature-branch", "", "レビュー対象のフィーチャーブランチ")

	cmd.Flags().StringVar(&cfg.LocalPath, "local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	cmd.Flags().StringVar(&cfg.GeminiModelName, "gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")

	cmd.Flags().StringVar(&cfg.SSHKeyPath, "ssh-key-path", "~/.ssh/id_rsa",
		"SSH認証に使用する秘密鍵ファイルのパス (デフォルト: ~/.ssh/id_rsa)")

	cmd.Flags().StringVar(&cfg.PromptFilePath, "prompt-file", "review_prompt.md",
		"Geminiへのレビュー依頼に使用するプロンプトファイルのパス")

	// --- 必須フラグの設定 ---
	// 必須フラグの設定は initCmdFlags の中で行うことで、init との関心事を分離できます。
	cmd.MarkFlagRequired("git-clone-url")
	cmd.MarkFlagRequired("base-branch")
	cmd.MarkFlagRequired("feature-branch")
}

func init() {
	// init()関数では、サブコマンドの追加とフラグの設定ヘルパー関数の呼び出しのみを行います。
	RootCmd.AddCommand(genericCmd)

	// 💡 Run関数内でローカル変数にバインドするため、initCmdFlags を呼び出す代わりに、
	// genericCmd のフラグ定義を initCmdFlags の内容で行うか、initCmdFlags の呼び出しを Run 関数内に移す。
	// ここでは、フラグ定義を initCmdFlags にまとめて、init 関数内で呼び出します。
	// これでコマンド実行前にフラグが正しく登録されます。

	// ダミーの cfg を作成してフラグを定義（値のバインドは Run で上書きされる）
	// ただし、Cobraの慣習としてフラグ定義は init で行うため、ここでは initCmdFlags の中身を直接展開します。

	// 🚀 initCmdFlags の中身を直接展開することで、グローバル変数を使わずにフラグを定義。
	// バインド先は *genericCmd* のローカルな cfg になるため、一時的にダミーの cfg を使うのではなく、
	// Run 関数内で localCfg を作成し、フラグの値を取得します。

	// 以下の行を削除し、initCmdFlags の内容を Run の PreRunE または Run の先頭に移動させるのが最もクリーンです。
	// しかし、Cobraの慣習としてフラグの定義は init で行うため、ここではフラグの定義部分のみを init に残します。

	// 💡 init 関数内では、Run 関数で利用するローカルな cfg へのバインドは行わず、
	// フラグの定義とデフォルト値の設定のみを行います。
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// フラグを定義
	genericCmd.Flags().String("git-clone-url", "", "レビュー対象のGitリポジトリURL")
	genericCmd.Flags().String("base-branch", "", "差分比較の基準ブランチ")
	genericCmd.Flags().String("feature-branch", "", "レビュー対象のフィーチャーブランチ")
	genericCmd.Flags().String("local-path", defaultLocalPath, fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))
	genericCmd.Flags().String("gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")
	genericCmd.Flags().String("ssh-key-path", "~/.ssh/id_rsa", "SSH認証に使用する秘密鍵ファイルのパス (デフォルト: ~/.ssh/id_rsa)")
	genericCmd.Flags().String("prompt-file", "review_prompt.md", "Geminiへのレビュー依頼に使用するプロンプトファイルのパス")

	// 必須フラグの設定
	genericCmd.MarkFlagRequired("git-clone-url")
	genericCmd.MarkFlagRequired("base-branch")
	genericCmd.MarkFlagRequired("feature-branch")
}

var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "Gitリポジトリの差分をレビューし、結果を標準出力します。",
	Long:  `このモードは、差分レビューの結果を標準出力に出力します。`,
	// RunE を使用することで、エラーを返せるようにします。
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// 1. ローカルな ReviewConfig インスタンスを作成し、フラグの値を設定
		var localCfg ReviewConfig

		// CobraのGetString/GetBoolなどを使ってフラグの値を取得し、localCfgに格納
		// (フラグの定義は init で完了しているため、ここで値を取得できます)
		var err error
		localCfg.GitCloneURL, err = cmd.Flags().GetString("git-clone-url")
		if err != nil {
			return err
		}
		localCfg.BaseBranch, err = cmd.Flags().GetString("base-branch")
		if err != nil {
			return err
		}
		localCfg.FeatureBranch, err = cmd.Flags().GetString("feature-branch")
		if err != nil {
			return err
		}
		localCfg.LocalPath, err = cmd.Flags().GetString("local-path")
		if err != nil {
			return err
		}
		localCfg.GeminiModelName, err = cmd.Flags().GetString("gemini-model-name")
		if err != nil {
			return err
		}
		localCfg.SSHKeyPath, err = cmd.Flags().GetString("ssh-key-path")
		if err != nil {
			return err
		}
		localCfg.PromptFilePath, err = cmd.Flags().GetString("prompt-file")
		if err != nil {
			return err
		}

		// 2. Gitクライアントを初期化し、リポジトリを処理
		gitClient := services.NewGitClient(localCfg.LocalPath, localCfg.SSHKeyPath)
		repo, err := gitClient.CloneOrOpen(localCfg.GitCloneURL)
		if err != nil {
			// エラーを直接返します。RootCmd.Execute()が stderr に出力し os.Exit(1) します。
			return fmt.Errorf("error processing repository: %w", err)
		}

		// 2.5. 最新の変更をフェッチ
		if err := gitClient.Fetch(repo); err != nil {
			return fmt.Errorf("error fetching latest changes: %w", err)
		}

		// 3. コード差分を取得
		codeDiff, err := gitClient.GetCodeDiff(repo, localCfg.BaseBranch, localCfg.FeatureBranch)
		if err != nil {
			return fmt.Errorf("error getting code diff: %w", err)
		}

		if codeDiff == "" {
			fmt.Println("レビュー対象の差分がありませんでした。処理を終了します。")
			return nil // 差分がない場合は成功として終了
		}

		fmt.Println("--- 差分取得完了。Geminiにレビューを依頼します... ---")

		// 4. Geminiクライアントを初期化
		geminiClient, err := services.NewGeminiClient(localCfg.GeminiModelName)
		if err != nil {
			return fmt.Errorf("error initializing Gemini client: %w", err)
		}
		defer geminiClient.Close() // defer は残します

		// 5. Geminiにレビューを依頼
		reviewResult, err := geminiClient.ReviewCodeDiff(ctx, codeDiff, localCfg.PromptFilePath)
		if err != nil {
			return fmt.Errorf("error requesting review from Gemini: %w", err)
		}

		// 6. 結果を標準出力
		fmt.Println("\n--- 📝 Gemini Code Review Result ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------------")

		return nil
	},
}
