package cmd

import (
	"fmt"
	"log/slog"

	"github.com/shouni/gemini-reviewer-core/pkg/publisher"
	"github.com/shouni/go-remote-io/pkg/factory"

	"github.com/spf13/cobra"
)

// GCSFlags は gcs コマンド固有のフラグを保持します。
type GCSFlags struct {
	GCSURI      string // GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.html)
	ContentType string // GCSに保存する際のMIMEタイプ
}

var gcsFlags GCSFlags

// gcsCmd は 'gcs' サブコマンドを定義します。
var gcsCmd = &cobra.Command{
	Use:   "gcs",
	Short: "AIレビュー結果をスタイル付きHTMLに変換し、その結果を指定されたGCS URIに保存します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をさらにAIでスタイル付きHTMLに変換した後、go-remote-io を利用してGCSにアップロードします。`,
	Args:  cobra.NoArgs,
	RunE:  gcsCommand,
}

func init() {
	gcsCmd.Flags().StringVarP(&gcsFlags.ContentType, "content-type", "t", "text/html; charset=utf-8", "GCSに保存する際のMIMEタイプ (デフォルトはHTML)")
	gcsCmd.Flags().StringVarP(&gcsFlags.GCSURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/review/result.html", "GCSの保存先")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// gcsCommand は gcs コマンドの実行ロジックです。
func gcsCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsFlags.GCSURI

	// 1. レビューパイプラインを実行
	reviewResult, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return err
	}

	if reviewResult == "" {
		slog.Warn("レビュー結果の内容が空のため、GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// 2. GCSへの結果保存
	ioFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return fmt.Errorf("クライアントファクトリの初期化に失敗しました: %w", err)
	}
	writer, err := publisher.NewGCSPublisher(ioFactory)
	if err != nil {
		return fmt.Errorf("クライアントファクトリの初期化に失敗しました: %w", err)
	}
	meta := publisher.ReviewMetadata{
		RepoURL:       ReviewConfig.RepoURL,
		BaseBranch:    ReviewConfig.BaseBranch,
		FeatureBranch: ReviewConfig.FeatureBranch,
	}
	err = writer.Publish(ctx, reviewResult, meta)
	if err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsFlags.GCSURI, err)
	}
	slog.Info("GCSへのアップロードが完了しました。", "uri", gcsFlags.GCSURI)

	return nil
}
