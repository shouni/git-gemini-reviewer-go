package cmd

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/shouni/go-remote-io/pkg/factory"
	mk2html "github.com/shouni/go-text-format/pkg/builder"
	"github.com/spf13/cobra"
)

// GcsSaveFlags は gcs-save コマンド固有のフラグを保持します。
type GcsSaveFlags struct {
	GCSURI      string // GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.html)
	ContentType string // GCSに保存する際のMIMEタイプ
}

var gcsSaveFlags GcsSaveFlags

const PromptTypeHTML = "html"

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
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.GCSURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/review/result.html", "GCSの保存先")
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
	gcsURI := gcsSaveFlags.GCSURI
	bucketName, objectPath, err := validateGcsURI(gcsURI)
	if err != nil {
		return err
	}

	// 1. AIレビューパイプラインを実行し、結果の文字列を受け取る
	slog.Info("Git/Geminiレビューパイプラインを実行中...")
	// executeReviewPipeline は cmd パッケージ内の他のファイルで定義されており、ReviewConfig は rootCmd で初期化されるグローバル変数です。
	slog.Info("Git/Geminiレビューパイプラインを実行中 (Markdown生成)...")
	reviewResultMarkdown, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return fmt.Errorf("レビューパイプラインの実行に失敗しました: %w", err)
	}

	// レビュー結果が空の場合は、警告を出して終了
	if reviewResultMarkdown == "" {
		slog.Warn("AIレビュー結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	htmlBuilder, err := mk2html.NewBuilder()
	if err != nil {
		slog.Error("HTML変換ビルダーの初期化に失敗しました。", "error", err)
		os.Exit(1)
	}

	converterService := htmlBuilder.ConverterService
	rendererService := htmlBuilder.RendererService
	htmlFragment, err := converterService.Convert(ctx, []byte(reviewResultMarkdown))
	if err != nil {
		return fmt.Errorf("HTMLフラグメント生成エラー: %w", err)
	}

	// ヘッダー文字列の作成 (ブランチ情報を結合)
	title := fmt.Sprintf(
		"AIコードレビュー結果 (ブランチ: `%s` ← `%s`)",
		ReviewConfig.BaseBranch,
		ReviewConfig.FeatureBranch,
	)
	var htmlBuffer bytes.Buffer
	err = rendererService.Render(&htmlBuffer, htmlFragment, "ja-jp", title)
	if err != nil {
		slog.Error("HTML変換プロンプトの組み立てエラー。", "error", err)
		return fmt.Errorf("HTML変換プロンプトの組み立てに失敗しました: %w", err)
	}

	// HTML変換結果が空文字列の場合のチェックを追加
	if htmlBuffer.Len() == 0 {
		slog.Warn("AIによるHTML変換結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// 4. ClientFactory の取得
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}

	// 5. GCSOutputWriter の取得
	writer, err := clientFactory.GetGCSOutputWriter()
	if err != nil {
		return fmt.Errorf("GCSOutputWriterの取得に失敗しました: %w", err)
	}

	// 7. レビュー結果文字列を io.Reader に変換
	contentReader := strings.NewReader(htmlBuffer.String())

	// 8. GCSへの書き込み実行
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsSaveFlags.ContentType)

	if err := writer.WriteToGCS(ctx, bucketName, objectPath, contentReader, gcsSaveFlags.ContentType); err != nil {
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsURI, err)
	}

	slog.Info("GCSへのアップロードが完了しました", "uri", gcsURI)

	return nil
}
