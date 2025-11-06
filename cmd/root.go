package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"
)

// --- グローバル変数 ---
var ReviewConfig config.ReviewConfig
var sharedClient *httpkit.Client

// --- 定数 ---
const defaultHTTPTimeout = 30 * time.Second

// --- 初期化ロジック ---

// initHTTPClient は共有の HTTP クライアントを初期化します。
func initHTTPClient() *httpkit.Client {
	return httpkit.New(defaultHTTPTimeout)
}

// initAppPreRunE は、アプリケーション固有のPersistentPreRunEです。
func initAppPreRunE(cmd *cobra.Command, args []string) error {

	// 1. slog ハンドラの設定
	logLevel := slog.LevelInfo
	if clibase.Flags.Verbose {
		logLevel = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{ // 標準エラー出力にログを出すのが一般的
		Level: logLevel,
	})
	slog.SetDefault(slog.New(handler))

	// 2. HTTPクライアントの初期化（グローバル変数に代入）
	sharedClient = initHTTPClient()
	slog.Debug("HTTPクライアントの初期化が完了しました。") // ログメッセージを移動

	// 3. レビューモードに基づき、プロンプトを含む設定を構築
	newConfig, err := CreateReviewConfig(ReviewConfig)
	if err != nil {
		// 設定構築エラーが発生した場合、処理を停止
		slog.Error("アプリケーション設定の初期化に失敗しました。", "error", err)
		return fmt.Errorf("application configuration initialization failed: %w", err)
	}

	// 更新された設定をグローバル変数に反映
	ReviewConfig = newConfig

	// Verbose ログ
	slog.Debug("アプリケーション設定", "config", ReviewConfig) // 修正済み

	// 設定完了ログ
	// 【修正 4】行番号 50: mode フィールドを削除
	slog.Info("アプリケーション設定初期化完了")

	return nil
}

// --- フラグ設定ロジック ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	// ReviewConfig.ReviewMode にバインド
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.ReviewMode,
		"mode",
		"m",
		"detail",
		"レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)",
	)
	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.GeminiModel,
		"model",
		"g",
		"gemini-2.5-flash",
		"Gemini model name to use for review (e.g., 'gemini-2.5-flash').",
	)
	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.GitCloneURL,
		"git-clone-url",
		"u",
		"",
		"The SSH URL of the Git repository to review.",
	)

	// 必須フラグのエラーハンドリング
	if err := rootCmd.MarkPersistentFlagRequired("git-clone-url"); err != nil {
		return
	}

	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.BaseBranch,
		"base-branch",
		"b",
		"main",
		"The base branch for diff comparison (e.g., 'main').",
	)
	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.FeatureBranch,
		"feature-branch",
		"f",
		"",
		"The feature branch to review (e.g., 'feature/my-branch').",
	)

	if err := rootCmd.MarkPersistentFlagRequired("feature-branch"); err != nil {
		return
	}

	// パスとホストキーチェックフラグ
	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.LocalPath,
		"local-path",
		"l",
		os.TempDir()+"/git-reviewer-repos/tmp",
		"Local path to clone the repository.",
	)
	rootCmd.PersistentFlags().StringVarP(
		&ReviewConfig.SSHKeyPath,
		"ssh-key-path",
		"k",
		"~/.ssh/id_rsa",
		"Path to the SSH private key for Git authentication.",
	)
	rootCmd.PersistentFlags().BoolVarP(
		&ReviewConfig.SkipHostKeyCheck,
		"skip-host-key-check",
		"s",
		false,
		"CRITICAL WARNING: Disables SSH host key verification. This dramatically increases the risk of Man-in-the-Middle attacks. NEVER USE IN PRODUCTION. Only for controlled development/testing environments.",
	)
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
