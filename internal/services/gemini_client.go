package services

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

// GeminiClient はGemini APIとの通信を管理します。
type GeminiClient struct {
	client    *genai.Client
	modelName string
}

// NewGeminiClient はGeminiClientを初期化します。
func NewGeminiClient(ctx context.Context, modelName string) (*GeminiClient, error) {
	// 1. APIキーの取得
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	// 2. クライアントの作成
	// SDKのバージョンアップに伴うAPI仕様の変更に対応するため、
	// genai.NewClient の引数を *genai.ClientConfig 形式に変更しています。
	clientConfig := &genai.ClientConfig{
		APIKey: apiKey,
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// ReviewCodeDiff はコード差分を基にGeminiにレビューを依頼します。
// promptTemplateString には、コード差分(%s)を埋め込むための fmt.Sprintf 形式のプレースホルダが含まれている必要があります。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, diffContent string, promptTemplateString string) (string, error) {

	// プロンプトの構成
	prompt := fmt.Sprintf(promptTemplateString, diffContent)
	// 入力コンテンツを作成
	contents := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: prompt},
			},
		},
	}

	// API呼び出しを実行 (want (context.Context, string, []*genai.Content, *genai.GenerateContentConfig) に準拠)
	resp, err := c.client.Models.GenerateContent(
		ctx,
		c.modelName, // 1st argument: モデル名 (string)
		contents,    // 2nd argument: コンテンツスライス ([]*genai.Content)
		// 3rd argument: コンフィグ (*genai.GenerateContentConfig)。今回はnilで省略可能だが、生成設定（温度、トークン制限など）が必要な場合に利用。
		nil,
	)

	if err != nil {
		return "", fmt.Errorf("GenerateContent failed with model %s: %w", c.modelName, err)
	}

	// レスポンスの処理
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("received empty or invalid response from Gemini API")
	}

	candidate := resp.Candidates[0]

	if candidate.FinishReason != genai.FinishReasonUnspecified && candidate.FinishReason != genai.FinishReasonStop {
		// FinishReason.String() が無い問題を回避するため、%v を使用
		return "", fmt.Errorf("API response was blocked or finished prematurely. Reason: %v", candidate.FinishReason)
	}

	// その後、コンテンツの有無をチェック
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("Gemini response candidate is empty or lacks content parts")
	}

	firstPart := candidate.Content.Parts[0]

	// Textフィールドの値を直接返す
	if firstPart.Text == "" {
		return "", fmt.Errorf("API returned non-text part in response or text field is empty")
	}

	return firstPart.Text, nil
}
