package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// RootCmd はアプリケーションのベースコマンド（"git-gemini-reviewer-go" 本体）です。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューするCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Gemini APIに渡してコードレビューを行います。

利用可能なサブコマンド:
  generic  (Backlog連携なし)
  backlog  (Backlog連携あり)`,
	// ベースコマンド自体は処理を持たず、サブコマンドへ処理を委譲します。
	Run: func(cmd *cobra.Command, args []string) {
		// 引数なしで実行された場合などにヘルプを表示
		cmd.Help()
	},
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// エラー発生時にエラーメッセージを出力し、終了コード1で終了
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
