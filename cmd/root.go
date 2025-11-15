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

	slog.Info("アプリケーション設定初期化完了", slog.String("mode", ReviewConfig.ReviewMode))

	return nil
}

// --- フラグ設定ロジック ---

// addAppPersistentFlags は、アプリケーション固有の永続フラグをルートコマンドに追加します。
func addAppPersistentFlags(rootCmd *cobra.Command) {
	// ReviewConfig.ReviewMode にバインド
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.ReviewMode, "mode", "m", "detail", "レビューモードを指定: 'release' (リリース判定) または 'detail' (詳細レビュー)")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.RepoURL, "repo-url", "u", "", "レビュー対象の Git リポジトリの SSH URL。")
	rootCmd.MarkPersistentFlagRequired("repo-url")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.BaseBranch, "base-branch", "b", "main", "差分比較の基準ブランチ (例: 'main').")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.FeatureBranch, "feature-branch", "f", "", "レビュー対象のフィーチャーブランチ (例: 'feature/my-branch').")
	rootCmd.MarkPersistentFlagRequired("feature-branch")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.LocalPath, "local-path", "l", "", "リポジトリをクローンするローカルパス。")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.GeminiModel, "gemini", "g", "gemini-2.5-flash", "レビューに使用する Gemini モデル名 (例: 'gemini-2.5-flash').")
	rootCmd.PersistentFlags().StringVarP(&ReviewConfig.SSHKeyPath, "ssh-key-path", "k", "~/.ssh/id_rsa", "Git 認証に使用する SSH 秘密鍵のパス。")
	rootCmd.PersistentFlags().BoolVar(&ReviewConfig.SkipHostKeyCheck, "skip-host-key-check", false, "【🚨 危険な設定】 SSH ホストキーの検証を無効にします。中間者攻撃のリスクを劇的に高めるため、本番環境では絶対に使用しないでください。開発/テスト環境でのみ使用してください。")
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
		gcsCmd,
	)
}
