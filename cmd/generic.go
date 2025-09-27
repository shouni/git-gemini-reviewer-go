package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// ReviewConfig は引数を保持する構造体です。
// ⚠️ 構造体は共通パッケージに切り出すことが推奨されますが、ここでは簡略化しています。
type ReviewConfig struct {
	GitCloneURL     string
	BaseBranch      string
	FeatureBranch   string
	LocalPath       string
	IssueID         string
	GeminiModelName string
}

var genericCfg ReviewConfig

func init() {
	// ルートコマンドにサブコマンドを登録
	RootCmd.AddCommand(genericCmd)

	// LocalPath のデフォルト値を設定
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")

	// --- フラグの定義 ---

	// 必須引数 (cobraでは MarkFlagRequired で必須化)
	genericCmd.Flags().StringVar(&genericCfg.GitCloneURL, "git-clone-url", "", "レビュー対象のGitリポジトリURL")
	genericCmd.MarkFlagRequired("git-clone-url") // 必須化

	genericCmd.Flags().StringVar(&genericCfg.BaseBranch, "base-branch", "", "差分比較の基準ブランチ")
	genericCmd.MarkFlagRequired("base-branch") // 必須化

	genericCmd.Flags().StringVar(&genericCfg.FeatureBranch, "feature-branch", "", "レビュー対象のフィーチャーブランチ")
	genericCmd.MarkFlagRequired("feature-branch") // 必須化

	// 任意の引数
	genericCmd.Flags().StringVar(&genericCfg.LocalPath, "local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	// Backlog連携がないため、IssueIDは任意
	genericCmd.Flags().StringVar(&genericCfg.IssueID, "issue-id", "", "関連する課題ID (レビュープロンプトのコンテキストに使用)")

	genericCmd.Flags().StringVar(&genericCfg.GeminiModelName, "gemini-model-name", "gemini-2.5-flash", "使用するGeminiモデル名")
}

var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "Backlog連携を行わず、結果を標準出力する汎用レビューモード",
	Long:  `このモードは、差分レビューの結果を標準出力にMarkdownとして出力します。`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- 実行ロジック ---
		fmt.Println("--- ✅ 汎用レビューモードを実行しました ---")
		fmt.Printf("設定:\n%+v\n", genericCfg)
		// ここで GitCodeReviewer(args) の実行を実装します。
	},
}
