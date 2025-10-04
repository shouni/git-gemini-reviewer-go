// cmd/root.go

package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

// --- パッケージレベル変数の定義 ---
var reviewMode string
var gitCloneURL string
var baseBranch string
var featureBranch string
var sshKeyPath string
var localPath string
var skipHostKeyCheck bool
var geminiModel string

// RootCmd はアプリケーションのベースコマンド（ディスパッチャ）です。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューするCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Gemini APIに渡してコードレビューを行います。

利用可能なサブコマンド:
  generic  (Backlog連携なし)
  backlog  (Backlog連携あり)`,

	// AIレビューの指摘に従い、Git Diff/AIレビューロジックを削除し、RunEをnilに戻します。
	// これにより、サブコマンドが指定されない場合、Cobraはヘルプを表示します。
	RunE: nil,
}

func init() {
	// PersistentFlags() でフラグを定義。第3引数がデフォルト値（"detail"）です。
	RootCmd.PersistentFlags().StringVarP(&reviewMode, "mode", "m", "detail", "レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)")

	// Git 関連のフラグを PersistentFlags として定義（サブコマンドすべてで使用可能に）
	RootCmd.PersistentFlags().StringVar(
		&gitCloneURL,
		"git-clone-url",
		"",
		"The SSH URL of the Git repository to review.",
	)
	RootCmd.PersistentFlags().StringVar(
		&baseBranch,
		"base-branch",
		"main",
		"The base branch for diff comparison (e.g., 'main').",
	)
	RootCmd.PersistentFlags().StringVar(
		&featureBranch,
		"feature-branch",
		"",
		"The feature branch to review (e.g., 'feature/my-branch').",
	)
	RootCmd.PersistentFlags().StringVar(
		&sshKeyPath,
		"ssh-key-path",
		"~/.ssh/id_rsa",
		"Path to the SSH private key for Git authentication.",
	)
	RootCmd.PersistentFlags().StringVar(
		&localPath,
		"local-path",
		os.TempDir() + "/git-reviewer-repos/tmp", // デフォルトの一時パス
		"Local path to clone the repository.",
	)
	RootCmd.PersistentFlags().BoolVar(
		&skipHostKeyCheck,
		"skip-host-key-check",
		false,
		"CRITICAL WARNING: Disables SSH host key verification. This dramatically increases the risk of Man-in-the-Middle attacks. NEVER USE IN PRODUCTION. Only for controlled development/testing environments.",
	)
	RootCmd.PersistentFlags().StringVar(
		&geminiModel,
		"model",
		"gemini-2.5-flash",
		"Gemini model name to use for review (e.g., 'gemini-2.5-flash').",
	)
	// 共通で必須となるフラグをルートコマンドでマーク
	RootCmd.MarkPersistentFlagRequired("git-clone-url")
	RootCmd.MarkPersistentFlagRequired("feature-branch")

	// NOTE: os.TempDir() を使うため、root.go に "os" をインポートする必要があります。
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}