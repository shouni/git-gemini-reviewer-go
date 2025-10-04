package services

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeminiClient はGemini APIとの通信を管理します。
type GeminiClient struct {
	client    *genai.Client
	modelName string
}

// NewGeminiClient はGeminiClientを初期化します。
func NewGeminiClient(modelName string) (*GeminiClient, error) {
	// 1. APIキーの取得
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	// 2. クライアントの作成
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// Close はクライアントを閉じ、リソースを解放します。
func (c *GeminiClient) Close() {
	if c.client != nil {
		// リソースのリークを防ぐためにクライアントをクローズ
		c.client.Close()
	}
}

// ReviewCodeDiff はコード差分を基にGeminiにレビューを依頼します。
//
// 修正点: promptTemplateString を引数として受け取るように変更しました。
// これにより、呼び出し元（cmd/root.go）で埋め込まれたプロンプトを直接渡せます。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, diffContent string, promptTemplateString string) (string, error) {

	// 1. プロンプトの構成
	// promptTemplateString をテンプレートとして使用し、コード差分を埋め込む
	// diffContentが %s のプレースホルダに確実に埋め込まれることを前提とします。
	prompt := fmt.Sprintf(promptTemplateString, diffContent)

	// 2. API呼び出し
	model := c.client.GenerativeModel(c.modelName)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("GenerateContent failed with model %s: %w", c.modelName, err)
	}

	// 3. レスポンスの処理 (元のロジックを維持)
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("received empty or invalid response from Gemini API")
	}

	candidate := resp.Candidates[0]

	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		if candidate.FinishReason != genai.FinishReasonUnspecified {
			return "", fmt.Errorf("API response was blocked or finished prematurely. Reason: %s", candidate.FinishReason.String())
		}
		return "", fmt.Errorf("Gemini response candidate is empty or lacks content parts")
	}

	// 4. テキスト内容の抽出
	reviewText, ok := candidate.Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("API returned non-text part in response")
	}

	result := string(reviewText)
	if result == "" {
		return "", fmt.Errorf("API returned an empty text review result")
	}

	return result, nil
}