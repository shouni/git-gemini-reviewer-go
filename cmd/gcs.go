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

// GCSFlags は gcs コマンド固有のフラグを保持します。
type GCSFlags struct {
	GCSURI      string // GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.html)
	ContentType string // GCSに保存する際のMIMEタイプ
}

var gcsFlags GCSFlags

// gcsSaveCmd は 'gcs' サブコマンドを定義します。
var gcsSaveCmd = &cobra.Command{
	Use:   "gcs",
	Short: "AIレビュー結果をスタイル付きHTMLに変換し、その結果を指定されたGCS URIに保存します。",
	Long: `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をさらにAIでスタイル付きHTMLに変換した後、go-remote-io を利用してGCSにアップロードします。
宛先 URI は '--gcs-uri' フラグで指定する必要があり、'gs://bucket-name/object-path' の形式である必要があります。`,
	Args: cobra.NoArgs,
	RunE: gcsSaveCommand,
}

func init() {
	gcsSaveCmd.Flags().StringVarP(&gcsFlags.ContentType, "content-type", "t", "text/html; charset=utf-8", "GCSに保存する際のMIMEタイプ (デフォルトはHTML)")
	gcsSaveCmd.Flags().StringVarP(&gcsFlags.GCSURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/review/result.html", "GCSの保存先")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runGcsSave は gcs コマンドの実行ロジックです。
func gcsSaveCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsFlags.GCSURI

	bucketName, objectPath, err := validateGcsURI(gcsURI)
	if err != nil {
		return err
	}

	// 1. レビューパイプラインを実行
	reviewResultMarkdown, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return err
	}

	if reviewResultMarkdown == "" {
		slog.Warn("AIレビュー結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// ヘッダー文字列の作成
	htmlTitle := fmt.Sprintf("AIコードレビュー結果")
	summaryMarkdown := fmt.Sprintf(
		"レビュー対象リポジトリ: `%s`\n\n**ブランチ差分:** `%s` ← `%s`\n\n",
		ReviewConfig.RepoURL,
		ReviewConfig.BaseBranch,
		ReviewConfig.FeatureBranch,
	)
	var combinedContentBuffer bytes.Buffer
	combinedContentBuffer.WriteString("## " + htmlTitle)
	combinedContentBuffer.WriteString("\n\n")
	// 要約情報をヘッダーの下に配置
	combinedContentBuffer.WriteString(summaryMarkdown)
	// レビュー結果の本文を追加
	combinedContentBuffer.WriteString(reviewResultMarkdown)

	// 2. HTML変換
	htmlBuffer, err := convertMarkdownToHTML(ctx, htmlTitle, combinedContentBuffer.Bytes())
	if err != nil {
		return fmt.Errorf("レビュー結果をHTML変換に失敗しました: %w", err)
	}

	// 3. レビュー結果をGCSへアップロードを実行
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsFlags.ContentType)
	err = uploadToGCS(ctx, bucketName, objectPath, htmlBuffer, gcsFlags.ContentType)
	if err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsFlags.GCSURI, err)
	}
	slog.Info("GCSへのアップロードが完了しました。")

	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// convertMarkdownToHTML Markdown形式の入力データを受け取り、HTML形式のデータに変換する。
func convertMarkdownToHTML(ctx context.Context, title string, reviewResultMarkdown []byte) (*bytes.Buffer, error) {
	htmlBuilder, err := builder.NewBuilder()
	if err != nil {
		return nil, err
	}

	mk2html, err := htmlBuilder.BuildMarkdownToHtmlRunner()
	if err != nil {
		return nil, err
	}

	return mk2html.ConvertMarkdownToHtml(ctx, title, reviewResultMarkdown)
}

// uploadToGCS はレンダリングされたHTMLをGCSにアップロードします。
func uploadToGCS(ctx context.Context, bucketName, objectPath string, content io.Reader, contentType string) error {
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}
	writer, err := clientFactory.GetGCSOutputWriter()
	if err != nil {
		return err
	}

	return writer.WriteToGCS(ctx, bucketName, objectPath, content, contentType)
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
