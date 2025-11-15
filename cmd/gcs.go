package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/shouni/go-remote-io/pkg/factory"
	"github.com/shouni/go-text-format/pkg/builder"
	"github.com/spf13/cobra"
)

// GcsSaveFlags は gcs-save コマンド固有のフラグを保持します。
type GcsSaveFlags struct {
	GcsURI      string // GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.html)
	ContentType string // GCSに保存する際のMIMEタイプ
}

var gcsSaveFlags GcsSaveFlags

// gcsSaveCmd は 'gcs-save' サブコマンドを定義します。
var gcsSaveCmd = &cobra.Command{
	Use:   "gcs",
	Short: "AIレビュー結果をスタイル付きHTMLに変換し、その結果を指定されたGCS URIに保存します。",
	Long: `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をさらにAIでスタイル付きHTMLに変換した後、go-remote-io を利用してGCSにアップロードします。
宛先 URI は '--gcs-uri' フラグで指定する必要があり、'gs://bucket-name/object-path' の形式である必要があります。`,
	Args: cobra.NoArgs,
	RunE: runGcsCommand,
}

func init() {
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.ContentType, "content-type", "t", "text/html; charset=utf-8", "GCSに保存する際のMIMEタイプ (デフォルトはHTML)")
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.GcsURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/review/result.html", "GCSの保存先")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runGcsSave は gcs-save コマンドの実行ロジックです。
func runGcsCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsSaveFlags.GcsURI

	bucketName, objectPath, err := validateGcsURI(gcsURI)
	if err != nil {
		return err
	}

	// 1. レビューパイプラインを実行
	reviewResultMarkdown, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return err
	}

	// ヘッダー文字列の作成 (ブランチ情報を結合)
	title := fmt.Sprintf(
		"AIコードレビュー結果 (ブランチ: `%s` ← `%s`)",
		ReviewConfig.BaseBranch,
		ReviewConfig.FeatureBranch,
	)

	// 2. HTML変換
	htmlBuffer, err := convertMarkdownToHtml(ctx, title, reviewResultMarkdown)
	if err != nil {
		return fmt.Errorf("レビュー結果をHTML変換に失敗しました。", err)
	}

	// 3. レビュー結果をGCSへアップロードを実行
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsSaveFlags.ContentType)
	err = uploadToGCS(ctx, bucketName, objectPath, htmlBuffer)
	if err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsURI, err)
	}
	slog.Info("GCSへのアップロードが完了しました。")

	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// convertMarkdownToHtml Markdown形式の入力データを受け取り、HTML形式のデータに変換する。
func convertMarkdownToHtml(ctx context.Context, title string, reviewResultMarkdown string) (*bytes.Buffer, error) {
	htmlBuilder, err := builder.NewBuilder()
	if err != nil {
		return nil, err
	}

	mk2html, err := htmlBuilder.BuildMarkdownToHtmlRunner()
	if err != nil {
		return nil, err
	}

	// タイトルとMarkdownコンテンツを結合
	var combinedContentBuffer bytes.Buffer
	combinedContentBuffer.WriteString("# " + title)
	combinedContentBuffer.WriteString("\n\n")
	combinedContentBuffer.WriteString(reviewResultMarkdown)

	return mk2html.ConvertMarkdownToHtml(ctx, title, combinedContentBuffer.Bytes())
}

// uploadToGCS はレンダリングされたHTMLをGCSにアップロードします。
func uploadToGCS(ctx context.Context, bucketName, objectPath string, content io.Reader) error {
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}
	writer, err := clientFactory.GetGCSOutputWriter()
	if err != nil {
		return err
	}

	return writer.WriteToGCS(ctx, bucketName, objectPath, content, gcsSaveFlags.ContentType)
}

// validateGcsURI は GCS URIの検証と解析を行うヘルパー関数です。
func validateGcsURI(gcsURI string) (bucketName, objectPath string, err error) {
	if !strings.HasPrefix(gcsURI, "gs://") {
		return "", "", fmt.Errorf("無効なGCS URIです。'gs://' で始まる必要があります: %s", gcsURI)
	}
	pathWithoutScheme := gcsURI[5:]
	parts := strings.SplitN(pathWithoutScheme, "/", 2)

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("無効なGCS URIフォーマットです。バケット名とオブジェクトパスが不足しています: %s", gcsURI)
	}
	return parts[0], parts[1], nil
}
