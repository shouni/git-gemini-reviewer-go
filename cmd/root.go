package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"git-gemini-reviewer-go/internal/config"

	"github.com/shouni/go-cli-base"
	"github.com/shouni/go-http-kit/pkg/httpkit"
	"github.com/spf13/cobra"
)

// ReviewConfig は、レビュー実行のパラメータです
var ReviewConfig config.ReviewConfig

const defaultHTTPTimeout = 30 * time.Second

// clientKey は context.Context に httpkit.Client を格納・取得するための非公開キー
type clientKey struct{}

// GetHTTPClient は、cmd.Context() から *httpkit.Client を取り出す公開関数です。
func GetHTTPClient(ctx context.Context) (*httpkit.Client, error) {
	if client, ok := ctx.Value(clientKey{}).(*httpkit.Client); ok {
		return client, nil
	}
	return nil, fmt.Errorf("contextからhttpkit.Clientを取得できませんでした。rootコマンドの初期化を確認してください。")
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

	// 2. HTTPクライアントの初期化
	httpClient := httpkit.New(defaultHTTPTimeout)

	// コマンドのコンテキストに HTTP Client を格納
	ctx := context.WithValue(cmd.Context(), clientKey{}, httpClient)
	cmd.SetContext(ctx)

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
	rootCmd.MarkPersistentFlagRequired("git-clone-url")

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
	rootCmd.MarkPersistentFlagRequired("feature-branch")

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
