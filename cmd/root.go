package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"
)

// --- パッケージレベル変数の定義 (Persistent Flags のバインド先) ---
var reviewMode string
var gitCloneURL string
var baseBranch string
var featureBranch string
var sshKeyPath string
var localPath string
var skipHostKeyCheck bool
var geminiModel string

// CreateReviewConfigParams は、フラグから読み取られたすべての引数を持つ構造体です。
// グローバル変数への依存を明示し、CreateReviewConfig に渡すために使用されます。
// NOTE: この構造体は、すべてのコマンドファイルからアクセスできるように、cmd パッケージ内で定義します。
type CreateReviewConfigParams struct {
	ReviewMode       string
	GeminiModel      string
	GitCloneURL      string
	BaseBranch       string
	FeatureBranch    string
	SSHKeyPath       string
	LocalPath        string
	SkipHostKeyCheck bool
}

// RootCmd はアプリケーションのベースコマンド（ディスパッチャ）です。
var RootCmd = &cobra.Command{
	Use:   "git-gemini-reviewer-go",
	Short: "Gemini AIを使ってGitの差分をレビューし、様々なプラットフォームに投稿するCLIツール",
	Long: `このツールは、指定されたGitブランチ間の差分を取得し、Google Gemini APIに渡してコードレビューを行います。

レビュー結果の出力先を選択できる3つのサブコマンドが利用可能です。

利用可能なサブコマンド:
  generic  : レビュー結果を標準出力 (STDOUT) に表示します。
  backlog  : レビュー結果をBacklogの課題コメントとして投稿します。
  slack    : レビュー結果をSlackの指定されたWebhook URLへ通知します。`,

	RunE: nil,
}

func init() {
	// PersistentFlags() でフラグを定義。
	RootCmd.PersistentFlags().StringVarP(&reviewMode, "mode", "m", "detail", "レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)")

	// Git 関連のフラグを PersistentFlags として定義
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
		os.TempDir()+"/git-reviewer-repos/tmp", // デフォルトの一時パス
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
}

// Execute はルートコマンドを実行し、アプリケーションを起動します。
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// log.Fatal の代わりに、エラーを出力し、os.Exit で終了する方がクリーン
		log.Println(err)
		os.Exit(1)
	}
}
