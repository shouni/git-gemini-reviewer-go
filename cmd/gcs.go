package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/shouni/go-remote-io/pkg/factory"
	mk2html "github.com/shouni/go-text-format/pkg/builder"
	"github.com/spf13/cobra"
)

// DefaultLocale はHTMLレンダリングに使用されるロケールです。
const DefaultLocale = "ja-jp"

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
	RunE: runGcsSave,
}

func init() {
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.ContentType, "content-type", "t", "text/html; charset=utf-8", "GCSに保存する際のMIMEタイプ (デフォルトはHTML)")
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.GcsURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/review/result.html", "GCSの保存先")
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

// runGcsSave は gcs-save コマンドの実行ロジックです。
func runGcsSave(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsSaveFlags.GcsURI

	bucketName, objectPath, err := validateGcsURI(gcsURI)
	if err != nil {
		return err
	}

	// 1. レビューパイプラインを実行し、HTMLドキュメントを bytes.Buffer にレンダリング
	htmlBuffer, err := convertMarkdownToHTML(ctx, gcsURI)
	if err != nil {
		return err
	}

	// convertMarkdownToHTML が nil を返した場合（スキップ処理）、エラーなしで終了する
	if htmlBuffer == nil {
		return nil
	}

	// 2. GCSへのアップロード
	return uploadToGCS(ctx, bucketName, objectPath, htmlBuffer, gcsURI)
}

// executeAndPrepareMarkdown はレビューパイプラインを実行し、ブランチ情報を付加したMarkdownと、生成されたタイトルを返します。
// 戻り値: (Markdownの []byte, タイトルの string, error)
func executeAndPrepareMarkdown(ctx context.Context, gcsURI string) (markdownContent []byte, title string, err error) {
	slog.Info("Git/Geminiレビューパイプラインを実行中 (Markdown生成)...")

	// executeReviewPipeline の戻り値は string であるため、そのまま受け取る
	reviewResultMarkdown, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return nil, "", fmt.Errorf("レビューパイプラインの実行に失敗しました: %w", err)
	}

	if reviewResultMarkdown == "" {
		slog.Warn("AIレビュー結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil, "", nil
	}

	// ヘッダー文字列の作成 (ブランチ情報を結合)
	title = fmt.Sprintf(
		"# AIコードレビュー結果 (ブランチ: `%s` ← `%s`)",
		ReviewConfig.BaseBranch,
		ReviewConfig.FeatureBranch,
	)

	// タイトルとMarkdownコンテンツを結合
	var combinedContentBuffer bytes.Buffer
	combinedContentBuffer.WriteString(title)
	combinedContentBuffer.WriteString("\n\n")
	combinedContentBuffer.WriteString(reviewResultMarkdown)

	// 戻り値を修正
	return combinedContentBuffer.Bytes(), title, nil
}

// convertMarkdownToHTML はMarkdownバイトスライスを受け取り、HTMLドキュメントを bytes.Buffer にレンダリングします。
// 戻り値: (*bytes.Buffer, error)
func convertMarkdownToHTML(ctx context.Context, gcsURI string) (*bytes.Buffer, error) {
	// 1. Markdownコンテンツとタイトルの取得と準備
	markdownToConvert, finalTitle, err := executeAndPrepareMarkdown(ctx, gcsURI)
	if err != nil {
		return nil, err
	}
	// executeAndPrepareMarkdown が nil を返した場合（スキップ処理）
	if len(markdownToConvert) == 0 {
		return nil, nil
	}

	// 2. ビルダーによるサービスの初期化
	htmlBuilder, err := mk2html.NewBuilder()
	if err != nil {
		slog.Error("HTML変換ビルダーの初期化に失敗しました。", "error", err)
		return nil, fmt.Errorf("HTML変換ビルダーの初期化に失敗しました: %w", err)
	}

	cService := htmlBuilder.ConverterService
	rService := htmlBuilder.RendererService

	// 3. MarkdownをHTMLフラグメントに変換
	htmlFragment, err := cService.Convert(ctx, markdownToConvert)
	if err != nil {
		return nil, fmt.Errorf("HTMLフラグメント生成エラー: %w", err)
	}

	// 4. HTMLドキュメントのレンダリング
	var htmlBuffer bytes.Buffer

	// finalTitle を `<title>` タグなどに使用してレンダリング
	err = rService.Render(&htmlBuffer, htmlFragment, DefaultLocale, finalTitle)
	if err != nil {
		slog.Error("HTMLレンダリングエラー。", "error", err)
		return nil, fmt.Errorf("HTMLレンダリングに失敗しました: %w", err)
	}

	if htmlBuffer.Len() == 0 {
		slog.Warn("HTMLレンダリング結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil, nil
	}

	return &htmlBuffer, nil
}

// uploadToGCS はレンダリングされたHTMLをGCSにアップロードします。
func uploadToGCS(ctx context.Context, bucketName, objectPath string, content io.Reader, gcsURI string) error {
	// 1. ClientFactory の取得
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}

	// 2. GCSOutputWriter の取得
	writer, err := clientFactory.GetGCSOutputWriter()
	if err != nil {
		return fmt.Errorf("GCSOutputWriterの取得に失敗しました: %w", err)
	}

	// 3. GCSへの書き込み実行
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsSaveFlags.ContentType)

	if err := writer.WriteToGCS(ctx, bucketName, objectPath, content, gcsSaveFlags.ContentType); err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsURI, err)
	}

	slog.Info("GCSへのアップロードが完了しました", "uri", gcsURI)

	return nil
}
