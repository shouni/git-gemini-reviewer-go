// cli/root.go
package cli

import (
	"fmt"
	"git-gemini-reviewer-go/internal"
	"github.com/spf13/cobra"
	"log"
	"os"
)

// RunEでフラグ変数を扱うための構造体を定義
type rootFlags struct {
	RepoName      string
	BaseBranch    string
	FeatureBranch string
	IssueID       string
	LocalPath     string
	ModelName     string
}

var flags = &rootFlags{}

var rootCmd = &cobra.Command{
	Use:   "backlog-reviewer-go [flags]", // 使用法を明確化
	Short: "BacklogのプルリクエストをレビューするためのCLIツール",
	Long:  `Python版の機能をGoに移植した、AIを活用したコードレビュー自動化ツールです。`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. ローカルパスの確定
		path, err := determineLocalPath(flags.LocalPath)
		if err != nil {
			return err
		}
		flags.LocalPath = path // 確定したパスを反映

		// 2. 引数の表示（ロギング）
		logReviewParameters(flags)

		// 3. ローカルパスの有効性チェック
		// ※ リポジトリをクローン/管理するディレクトリは存在する必要がある
		if err := validateLocalPath(flags.LocalPath); err != nil {
			return err
		}

		// 4. ビジネスロジックの起動
		params := internal.ReviewParams{
			RepoName:      flags.RepoName,
			BaseBranch:    flags.BaseBranch,
			FeatureBranch: flags.FeatureBranch,
			IssueID:       flags.IssueID,
			LocalPath:     flags.LocalPath,
			ModelName:     flags.ModelName,
		}

		return internal.RunReviewer(params)
	},
}

// ----------------------------------------------------
// 公開関数
// ----------------------------------------------------

func Execute() error {
	return rootCmd.Execute()
}

// ----------------------------------------------------
// 初期化とフラグ設定
// ----------------------------------------------------

func init() {
	// -r/--repo-name (デフォルト: APP)
	rootCmd.Flags().StringVarP(&flags.RepoName, "repo-name", "r", "APP", "対象リポジトリを含むプロジェクトキー (例: APP-101 の 'APP')")
	// -m/--base-branch (デフォルト: master)
	rootCmd.Flags().StringVarP(&flags.BaseBranch, "base-branch", "m", "master", "比較の基準となるブランチ名 (ベースブランチ)")
	// -f/--feature-branch (デフォルト: develop)
	rootCmd.Flags().StringVarP(&flags.FeatureBranch, "feature-branch", "f", "develop", "レビュー対象のフィーチャーブランチ名 (必須)")
	// -i/--issue-id (必須フラグ)
	rootCmd.Flags().StringVarP(&flags.IssueID, "issue-id", "i", "", "Backlogの課題ID (例: APP-101) (必須)")
	// -p/--local-path (デフォルト: カレントディレクトリ)
	rootCmd.Flags().StringVarP(&flags.LocalPath, "local-path", "p", "", "リポジトリのクローン先/作業ディレクトリ (デフォルト: カレントディレクトリ)")
	// -g/--gemini-model-name (デフォルト: gemini-2.0-flash)
	rootCmd.Flags().StringVarP(&flags.ModelName, "gemini-model-name", "g", "gemini-2.0-flash", "コードレビューに使用するGeminiのモデル名。")

	// 必須フラグの設定
	if err := rootCmd.MarkFlagRequired("issue-id"); err != nil {
		log.Fatal(err)
	}
	if err := rootCmd.MarkFlagRequired("feature-branch"); err != nil {
		log.Fatal(err)
	}
}

// ----------------------------------------------------
// ヘルパー関数 (ロジックの分離)
// ----------------------------------------------------

// determineLocalPath は、指定されたパスが空の場合にカレントディレクトリを返します。
func determineLocalPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	// パスが空の場合はカレントディレクトリを使用
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("カレントディレクトリの取得に失敗しました: %w", err)
	}
	return wd, nil
}

// validateLocalPath は、指定されたパスが有効なディレクトリであるかチェックします。
func validateLocalPath(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("エラー: 指定されたパス '%s' は存在しません。", path)
	}
	if err != nil {
		// その他のStatエラー
		return fmt.Errorf("パス '%s' の情報取得に失敗しました: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("エラー: 指定されたパス '%s' はディレクトリではありません。", path)
	}
	return nil
}

// logReviewParameters は、設定されたパラメータをロギングします。
func logReviewParameters(f *rootFlags) {
	log.Println("--- レビューを開始します ---")
	log.Printf("リポジトリ名 (プロジェクトキー): %s", f.RepoName) // ログ出力をより正確に
	log.Printf("ベースブランチ: %s", f.BaseBranch)
	log.Printf("レビューブランチ: %s", f.FeatureBranch)
	log.Printf("課題ID: %s", f.IssueID)
	log.Printf("ローカルパス: %s", f.LocalPath)
	log.Printf("使用するGeminiモデル: %s", f.ModelName)
}
