package services

import (
	"context"
	_ "embed"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

var ReviewPromptTemplate string

/*
	【重要】もし `no matching files found` エラーが出る場合:
	`review_prompt.md` ファイルが、この `gemini_client.go` と同じディレクトリ
	(つまり `internal/services/`) に存在しているか確認してください。
*/

// テンプレートを利用する例
func GetFilledReviewPrompt(diffContent string) string {
	// テンプレート変数はすでにビルド時に埋め込まれているので、そのまま fmt.Sprintf で利用できる
	finalPrompt := fmt.Sprintf(ReviewPromptTemplate, diffContent)
	return finalPrompt
}

// GeminiClient は Gemini API へのアクセスを管理
type GeminiClient struct {
	ModelName string
	client    *genai.Client
}

// NewGeminiClient は GeminiClient の新しいインスタンスを作成します。
// 環境変数 GEMINI_API_KEY からキーを読み込みます。
func NewGeminiClient(modelName string) (*GeminiClient, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Gemini client: %w", err)
	}

	return &GeminiClient{
		ModelName: modelName,
		client:    client,
	}, nil
}

// Close は基になるGeminiクライアント接続をクローズします。
// アプリケーション終了時に呼び出す必要があります。
func (c *GeminiClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// ReviewCodeDiff はコードの差分を受け取り、Geminiにレビューを依頼します。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, codeDiff string) (string, error) {
	// AIに渡すためのプロンプトを構築
	prompt := fmt.Sprintf(ReviewPromptTemplate, codeDiff)

	// モデルにリクエストを送信
	resp, err := c.client.GenerativeModel(c.ModelName).GenerateContent(
		ctx,
		genai.Text(prompt),
	)

	if err != nil {
		return "", fmt.Errorf("failed to request review from Gemini API: %w", err)
	}

	// レスポンスからテキストを安全に抽出
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("Gemini returned an invalid or empty response")
	}

	if textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
		return string(textPart), nil
	}

	return "", fmt.Errorf("Gemini response content was not in expected text format")
}
