package cmd

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"git-gemini-reviewer-go/internal/builder"
	"git-gemini-reviewer-go/prompts"

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
	Use:   "gcs",
	Short: "AIレビュー結果を実行し、その結果を指定されたGCS URIに保存します。",
	Long: `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をgo-remote-io を利用してGCSにアップロードします。
宛先 URI は '--gcs-uri' フラグで指定する必要があり、'gs://bucket-name/object-path' の形式である必要があります。`,
	Args: cobra.NoArgs,
	RunE: runGcsSave,
}

func init() {
	// フラグの初期化
	gcsSaveCmd.Flags().StringVarP(&gcsSaveFlags.ContentType, "content-type", "t", "text/html; charset=utf-8", "GCSに保存する際のMIMEタイプ")
	gcsSaveCmd.Flags().StringVar(&gcsSaveFlags.GCSURI, "gcs-uri", "gs://git-gemini-reviewer-go/ReviewResult/result.html", "GCSへ保存する際の宛先URI (例: gs://bucket/path/to/result.html)")
}

// runGcsSave は gcs-save コマンドの実行ロジックです。
func runGcsSave(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	gcsURI := gcsSaveFlags.GCSURI

	// 1. AIレビューパイプラインを実行し、結果の文字列を受け取る
	slog.Info("Git/Geminiレビューパイプラインを実行中...")
	// executeReviewPipeline の定義は外部にある前提。ReviewConfig の初期化・定義元を明確にすべき (指摘35)
	reviewResultMarkdown, err := executeReviewPipeline(ctx, ReviewConfig)
	if err != nil {
		return fmt.Errorf("レビューパイプラインの実行に失敗しました: %w", err)
	}

	// レビュー結果が空の場合は、警告を出して終了
	if reviewResultMarkdown == "" {
		slog.Warn("AIレビュー結果が空文字列でした。GCSへの保存をスキップします。", "uri", gcsURI)
		return nil
	}

	// 2. Gemini Clientの取得
	geminiService, err := builder.BuildGeminiService(ctx, ReviewConfig)
	if err != nil {
		return fmt.Errorf("Gemini Serviceの構築に失敗しました: %w", err)
	}

	// 3. 第二のAI呼び出し: Markdownをスタイル付きHTMLに変換
	slog.Info("レビュー結果のMarkdownをスタイル付きHTMLに変換中...", "model", ReviewConfig.GeminiModel)

	// 💡 修正: ReviewPromptBuilder を使用して構造的にプロンプトを組み立てる (指摘50に対応)
	// prompts.HTMLPromptTemplate の内容に "%s" が含まれていることを前提とする
	htmlPromptBuilder, err := prompts.NewReviewPromptBuilder("html", prompts.HTMLPromptTemplate)
	if err != nil {
		slog.Error("HTMLプロンプトビルダーの初期化エラー。", "error", err)
		return fmt.Errorf("HTMLプロンプトビルダーの初期化に失敗しました: %w", err)
	}

	reviewData := prompts.ReviewTemplateData{
		DiffContent: reviewResultMarkdown, // Markdown結果をDiffContentとしてデータに渡す
	}

	// Buildメソッドが内部でテンプレートを安全に処理する
	finalPrompt, err := htmlPromptBuilder.Build(reviewData)
	if err != nil {
		slog.Error("HTML変換プロンプトの組み立てエラー。", "error", err)
		return fmt.Errorf("HTML変換プロンプトの組み立てに失敗しました: %w", err)
	}

	// AIにHTMLを生成させる
	htmlResult, err := geminiService.GenerateText(ctx, finalPrompt)
	if err != nil {
		return fmt.Errorf("GeminiによるHTML生成に失敗しました: %w", err)
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

	// 6. URIをバケット名とオブジェクトパスに分離し、検証
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

	// 7. レビュー結果文字列を io.Reader に変換
	contentReader := strings.NewReader(htmlResult)

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
