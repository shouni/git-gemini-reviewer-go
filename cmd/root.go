package cmd

import (
	_ "embed" // embed パッケージのインポート（使用しないがディレクティブのために必要）
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// --- 埋め込みプロンプトの定義 ---
// 重要な点: コマンドの実行ロジックよりも前に、パッケージレベルで定義が必要です。
// ファイルパスは、このファイル (cmd/root.go) からの相対パスに合わせて調整してください。

//go:embed prompts/release_review_prompt.md
var releasePrompt string

//go:embed prompts/detail_review_prompt.md
var detailPrompt string

// --- パッケージレベル変数の定義 ---
// フラグの値を受け取るための変数をパッケージレベルで定義します。
var reviewMode string

// RootCmd はアプリケーションのベースコマンドです。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューするCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Gemini APIに渡してコードレビューを行います。

利用可能なサブコマンド:
  generic  (Backlog連携なし)
  backlog  (Backlog連携あり)`,

	// RunE を使用して、Execute() ではなくコマンド実行時にロジックを実行します。
	// 今回の用途では、選択ロジックを Execute() からこちらに移すことで整理できます。
	RunE: func(cmd *cobra.Command, args []string) error {
		var selectedPrompt string

		// 選択されたモードに基づいてプロンプトを決定します
		switch reviewMode {
		case "release":
			selectedPrompt = releasePrompt
			fmt.Println("✅ リリースレビューモードが選択されました。")
		case "detail":
			selectedPrompt = detailPrompt
			fmt.Println("✅ 詳細レビューモードが選択されました。")
		default:
			// フラグのバリデーションは、Execute() ではなくここで実行します
			return fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
		}

		// 選択されたプロンプトを使用します（実際はここでレビューロジックを呼び出します）
		fmt.Printf("--- 選択されたプロンプトの内容（プレビュー）---\n%s\n", selectedPrompt)

		// 通常はここで diff を取得し、AI処理ロジックを呼び出します。
		// デモとしてnilを返して正常終了とします。
		return nil
	},
}

// init() 関数は、パッケージがインポートされたときに自動的に実行されます。
// ここで Cobra のフラグ設定を行います。
func init() {
	// PersistentFlags() を使って、このルートコマンドと全てのサブコマンドで利用可能なフラグを定義します。
	RootCmd.PersistentFlags().StringVarP(&reviewMode, "mode", "m", "release", "レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)")

	// 注: 標準の Go の 'flag' パッケージは、'cobra' を使う場合は通常使いません。
	// 競合を避けるため、元のコードにあった flag.Parse() も削除しました。
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// エラー発生時にエラーメッセージを出力し、終了コード1で終了
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
