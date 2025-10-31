package cmd

import (
	"log"
	"os"

	"github.com/shouni/go-cli-base"
	request "github.com/shouni/go-web-exact/v2/pkg/client"
	"github.com/spf13/cobra"
)

// --- アプリケーション固有のフラグを保持する構造体 ---

var sharedClient *request.Client

// AppFlags は git-gemini-reviewer-go 固有の永続フラグを保持します。
type AppFlags struct {
	ReviewMode       string
	GeminiModel      string
	GitCloneURL      string
	BaseBranch       string
	FeatureBranch    string
	SSHKeyPath       string
	LocalPath        string
	SkipHostKeyCheck bool
}

// Flags はアプリケーション固有フラグにアクセスするためのグローバル変数
var Flags AppFlags

// CreateReviewConfigParams は、フラグから読み取られたすべての引数を持つ構造体です。
// NOTE: 固有フラグのグローバル変数 (Flags AppFlags) から値をコピーし、
// この構造体をロジック層に渡すことを意図していると解釈します。
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

// --- clibase に渡すカスタム関数 ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
// clibase.CustomFlagFunc のシグネチャに合致します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&Flags.ReviewMode,
		"mode",
		"m",
		"detail",
		"レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)",
	)
	rootCmd.PersistentFlags().StringVarP(
		&Flags.GeminiModel,
		"gemini-model",
		"G", // '--mode' の 'm' との競合を避けるため 'G' を使用
		"gemini-2.5-flash",
		"Gemini model name to use for review (e.g., 'gemini-2.5-flash').",
	)
	rootCmd.PersistentFlags().StringVarP(
		&Flags.GitCloneURL,
		"git-clone-url",
		"u",
		"",
		"The SSH URL of the Git repository to review.",
	)
	rootCmd.MarkPersistentFlagRequired("git-clone-url")

	rootCmd.PersistentFlags().StringVarP(
		&Flags.BaseBranch,
		"base-branch",
		"b",
		"main",
		"The base branch for diff comparison (e.g., 'main').",
	)
	rootCmd.PersistentFlags().StringVarP(
		&Flags.FeatureBranch,
		"feature-branch",
		"f",
		"",
		"The feature branch to review (e.g., 'feature/my-branch').",
	)
	rootCmd.MarkPersistentFlagRequired("feature-branch")

	rootCmd.PersistentFlags().StringVarP(
		&Flags.LocalPath,
		"local-path",
		"l",
		os.TempDir()+"/git-reviewer-repos/tmp",
		"Local path to clone the repository.",
	)
	rootCmd.PersistentFlags().StringVarP(
		&Flags.SSHKeyPath,
		"ssh-key-path",
		"k",
		"~/.ssh/id_rsa",
		"Path to the SSH private key for Git authentication.",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&Flags.SkipHostKeyCheck,
		"skip-host-key-check",
		"s",
		false,
		"CRITICAL WARNING: Disables SSH host key verification. This dramatically increases the risk of Man-in-the-Middle attacks. NEVER USE IN PRODUCTION. Only for controlled development/testing environments.",
	)

}

// initAppPreRunE は、clibase共通処理の後に実行される、アプリケーション固有のPersistentPreRunEです。
// 現状、特別な初期化がないため、nilを返します。
// clibase.CustomPreRunEFunc のシグネチャに合致します。
func initAppPreRunE(cmd *cobra.Command, args []string) error {
	// clibase.Flags.Verbose などにアクセス可能
	if clibase.Flags.Verbose {
		log.Printf("Verbose mode: clibase flags: %+v, app flags: %+v", clibase.Flags, Flags)
	}

	// ここにアプリケーション固有の実行前チェック（例：ファイル存在チェック、環境変数チェックなど）を記述
	return nil
}

// --- エントリポイント ---

// Execute は、clibase.Execute を使用してルートコマンドの構築と実行を委譲します。
func Execute() {
	clibase.Execute(
		"git-gemini-reviewer-go",
		addAppPersistentFlags,
		initAppPreRunE,
		genericCmd,
		backlogCmd,
		slackCmd,
	)
}
