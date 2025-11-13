package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/shouni/go-remote-io/pkg/factory"
)

// GcsSaveFlags は gcs-save コマンド固有のフラグを保持します。
type GcsSaveFlags struct {
	GCSURI      string // --gcs-uri 宛先 GCS URI (例: gs://bucket/path/to/result.md)
	ContentType string // --content-type GCSに保存する際のMIMEタイプ
}

var gcsSaveFlags GcsSaveFlags

// gcsSaveCmd は 'gcs-save' サブコマンドを定義します。
var gcsSaveCmd = &cobra.Command{
	Use:   "gcs-save",
	Short: "AIレビュー結果を実行し、その結果を指定されたGCS URIに保存します。",
	Long: `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をgo-remote-io を利用してGCSにアップロードします。
宛先 URI は '--gcs-uri' フラグで指定する必要があり、'gs://bucket-name/object-path' の形式である必要があります。`,
	Args: cobra.NoArgs,
	RunE: runGcsSave,
}

func init() {
	// フラグの初期化
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.ContentType, "content-type", "t", "text/markdown; charset=utf-8", "GCSに保存する際のMIMEタイプ")
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.GCSURI, "gcs-uri", "s", "gs://git-gemini-reviewer-go/ReviewResult/result.md", "GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.md)")
}

// runGcsSave は gcs-save コマンドの実行ロジックです。
func runGcsSave(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsSaveFlags.GCSURI

	// 1. AIレビューパイプラインを実行し、結果の文字列を受け取る
	slog.Info("Git/Geminiレビューパイプラインを実行中...")
	// executeReviewPipeline と ReviewConfig はこのパッケージ内の他のファイルで定義されている前提
	reviewResult, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return fmt.Errorf("レビューパイプラインの実行に失敗しました: %w", err)
	}

	// レビュー結果が空の場合は、警告を出して終了
	if reviewResult == "" {
		slog.Warn("AIレビュー結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// 2. ClientFactory の取得
	// NOTE: rootCmdでfactoryがContextに注入されている場合、NewClientFactory(ctx)ではなくGetClientFactory(ctx)を使用すべき
	clientFactory, err := factory.NewClientFactory(ctx)
	if err != nil {
		return err
	}

	// 3. GCSOutputWriter の取得
	writer, err := clientFactory.GetGCSOutputWriter()
	if err != nil {
		return fmt.Errorf("GCSOutputWriterの取得に失敗しました: %w", err)
	}

	// 4. URIをバケット名とオブジェクトパスに分離し、検証 (ロジックは前回修正を維持)
	if !strings.HasPrefix(gcsURI, "gs://") {
		return fmt.Errorf("無効なGCS URIです。'gs://' で始まる必要があります: %s", gcsURI)
	}
	pathWithoutScheme := gcsURI[5:]
	parts := strings.SplitN(pathWithoutScheme, "/", 2)

	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("無効なGCS URIフォーマットです。バケット名とオブジェクトパスが不足しています: %s", gcsURI)
	}

	bucketName := parts[0]
	objectPath := parts[1]

	// 5. レビュー結果文字列を io.Reader に変換
	contentReader := strings.NewReader(reviewResult)

	// 6. GCSへの書き込み実行 (io.Reader を渡す)
	// 修正: slog.Info を使用し、構造化されたロギングに置き換える
	slog.Info("レビュー結果をGCSへアップロード中",
		"uri", gcsURI,
		"bucket", bucketName,
		"object", objectPath,
		"content_type", gcsSaveFlags.ContentType)

	if err := writer.WriteToGCS(ctx, bucketName, objectPath, contentReader, gcsSaveFlags.ContentType); err != nil {
		// エラーログは呼び出し元で処理されるが、詳細なエラーを返す
		return fmt.Errorf("GCSへの書き込みに失敗しました (URI: %s): %w", gcsURI, err)
	}

	// 修正: slog.Info を使用し、構造化されたロギングに置き換える
	slog.Info("GCSへのアップロードが完了しました", "uri", gcsURI)

	return nil
}
