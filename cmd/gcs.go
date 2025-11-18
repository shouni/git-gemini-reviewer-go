package cmd

import (
	"bytes"
	"context"
	"fmt"
	"git-gemini-reviewer-go/internal/config"
	"io"
	"log/slog"
	"strings"

	"git-gemini-reviewer-go/pkg/adapters"

	"github.com/shouni/go-remote-io/pkg/factory"
	"github.com/shouni/go-remote-io/pkg/remoteio"

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
	bucketName, objectPath, err := remoteio.ParseGCSURI(gcsURI)
	if err != nil {
		return err
	}

	// 1. レビューパイプラインを実行
	reviewResult, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return err
	}

	if reviewResult == "" {
		slog.Warn("レビュー結果の内容が空のため、GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// 2. HTML変換
	finalHTML, err := convertMarkdownToHTML(ctx, reviewResult, ReviewConfig)
	if err != nil {
		return fmt.Errorf("レビュー結果をHTML変換に失敗しました: %w", err)
	}

	// 3. レビュー結果をGCSへアップロードを実行
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsFlags.ContentType)

	// 4. GCSへの結果保存
	err = uploadToGCS(ctx, bucketName, objectPath, strings.NewReader(finalHTML), gcsFlags.ContentType)
	if err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsFlags.GCSURI, err)
	}
	slog.Info("GCSへのアップロードが完了しました。", "uri", gcsFlags.GCSURI)

	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// convertMarkdownToHTML Markdown形式の入力データを受け取り、HTML形式のデータに変換する。
func convertMarkdownToHTML(ctx context.Context, reviewMarkdown string, opt config.ReviewConfig) (string, error) {
	markdownRunner, err := adapters.NewMarkdownToHtmlRunner(ctx)
	if err != nil {
		return "", fmt.Errorf("MarkdownToHtmlRunner の構築に失敗しました: %w", err)
	}

	// ヘッダー文字列の作成
	summaryMarkdown := fmt.Sprintf(
		"レビュー対象リポジトリ: `%s`\n\nブランチ差分: `%s` ← `%s`\n\n",
		opt.RepoURL,
		opt.BaseBranch,
		opt.FeatureBranch,
	)
	var combinedContentBuffer bytes.Buffer
	combinedContentBuffer.WriteString("## " + adapters.ReviewTitle)
	combinedContentBuffer.WriteString("\n\n")
	// 要約情報をヘッダーの下に配置
	combinedContentBuffer.WriteString(summaryMarkdown)
	combinedContentBuffer.WriteString("\n\n")
	// レビュー結果の本文を追加
	combinedContentBuffer.WriteString(reviewMarkdown)

	slog.Debug("MarkdownToHtmlRunner.Run を実行します。")
	return markdownRunner.Run(ctx, combinedContentBuffer.Bytes())
}

// uploadToGCS はレンダリングされたHTMLをGCSにアップロードします。
func uploadToGCS(ctx context.Context, bucketName, objectPath string, content io.Reader, contentType string) error {
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}
	writer, err := clientFactory.NewOutputWriter()
	if err != nil {
		return fmt.Errorf("GCSOutputWriterの取得に失敗しました: %w", err)
	}

	return writer.WriteToGCS(ctx, bucketName, objectPath, content, contentType)
}
