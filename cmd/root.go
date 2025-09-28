package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ReviewConfig はコマンドライン引数を保持する構造体です。
// main パッケージで定義されたものと同一ですが、利便性のためここではフィールドを再定義しています。
// 依存性管理のため、実際には main パッケージの ReviewConfig をインポートして使用するのが望ましいです。
// (今回は提示されたコードのスコープ内で構造体を整理しました)
type ReviewConfig struct {
	GitCloneURL     string
	BaseBranch      string
	FeatureBranch   string
	LocalPath       string
	IssueID         string
	GeminiModelName string
	SSHKeyPath      string
	PromptFilePath  string
}

// RootCmd はアプリケーションのベースコマンド（"git-gemini-reviewer-go" 本体）です。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューするCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Gemini APIに渡してコードレビューを行います。

利用可能なサブコマンド:
  generic  (Backlog連携なし)
  backlog  (Backlog連携あり)`,
	// 💡 修正点: ベースコマンド自体にロジックを持たせないため、Run は nil にします。
	// サブコマンドが存在する場合、引数なしで実行すると Cobra が自動でヘルプを表示します。
	Run: nil,
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// エラー発生時にエラーメッセージを出力し、終了コード1で終了
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
