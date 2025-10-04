package cmd

import (
	_ "embed"
	"context"
	"fmt"
	"os"
	"os/exec"

	"git-gemini-reviewer-go/internal/services"

	"github.com/spf13/cobra"
)

//go:embed prompts/release_review_prompt.md
var releasePrompt string

//go:embed prompts/detail_review_prompt.md
var detailPrompt string

// --- パッケージレベル変数の定義 ---
var reviewMode string

// RootCmd はアプリケーションのベースコマンドです。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューするCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Gemini APIに渡してコードレビューを行います。

利用可能なサブコマンド:
  generic  (Backlog連携なし)
  backlog  (Backlog連携あり)`,

	// RunE はコマンド実行時に呼び出されます。
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. レビューモードに基づいたプロンプトの選択
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

		// 2. Git Diff の取得
		// 例: 現在のブランチと 'HEAD^' (直前のコミット) との差分を取得
		fmt.Println("🔍 Gitの差分を取得中...")
		// 注: HEAD^ (直前のコミット) と HEAD (現在のコミット/ワーキングディレクトリ) の差分を取得
		diffCmd := exec.Command("git", "diff", "HEAD^", "HEAD")
		output, err := diffCmd.Output()
		if err != nil {
			// git diff が差分を見つけられなかった場合の特殊なエラー処理
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				// 差分がない場合、Git diff は終了コード1を返すことがある
				fmt.Println("ℹ️ 差分が見つかりませんでした。レビューをスキップします。")
				return nil
			}
			return fmt.Errorf("Git diff の実行に失敗しました: %w", err)
		}
		diffContent := string(output)

		if len(diffContent) == 0 {
			fmt.Println("ℹ️ 差分が見つかりませんでした。レビューをスキップします。")
			return nil
		}

		// 3. Gemini クライアントの初期化
		// モデル名を指定し、APIキーは services.NewGeminiClient 内で環境変数から取得されます。
		const geminiModel = "gemini-2.5-flash" // 高速な flash モデルを使用
		client, err := services.NewGeminiClient(geminiModel)
		if err != nil {
			return fmt.Errorf("Geminiクライアントの初期化に失敗しました: %w", err)
		}
		defer client.Close() // 関数終了時にクライアントを閉じる

		// 4. Gemini AIにレビューを依頼
		fmt.Println("🚀 Gemini AIによるコードレビューを開始します...")
		// 以前修正した services.ReviewCodeDiff のシグネチャに合わせる
		reviewResult, err := client.ReviewCodeDiff(context.Background(), diffContent, selectedPrompt)
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

// init() 関数は、パッケージがインポートされたときに自動的に実行されます。
// ここで Cobra のフラグ設定を行います。
func init() {
	// PersistentFlags() を使って、ルートコマンドと全てのサブコマンドで利用可能なフラグを定義します。
	// デフォルトを 'detail' に設定します。
	RootCmd.PersistentFlags().StringVarP(&reviewMode, "mode", "m", "detail", "レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)")
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
