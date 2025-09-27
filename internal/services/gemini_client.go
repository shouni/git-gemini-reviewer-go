package services

import (
	"context"
	"fmt"
	"io/ioutil"
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
		c.client.Close()
	}
}

// ReviewCodeDiff はコード差分を基にGeminiにレビューを依頼します。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, codeDiff string, promptFilePath string) (string, error) {
	// 1. プロンプトファイルの読み込み
	promptTemplate, err := ioutil.ReadFile(promptFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %s: %w", promptFilePath, err)
	}

	// 2. プロンプトの構成
	prompt := fmt.Sprintf(string(promptTemplate), codeDiff)

	// 3. API呼び出し
	resp, err := c.client.GenerativeModel(c.modelName).GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("GenerateContent failed: %w", err)
	}

	// 4. レスポンスの処理
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "レビュー結果を取得できませんでした。レスポンスが空です。", nil
	}

	if len(resp.Candidates[0].Content.Parts) == 0 {
		return "レビュー結果を取得できませんでした。コンテンツが空です。", nil
	}

	// genai.Part の型アサーションを使用してテキストを安全に取り出す
	part := resp.Candidates[0].Content.Parts[0]

	reviewTextPart, ok := part.(genai.Text)

	if !ok {
		// テキストでない場合（画像などが返された場合）
		return "レビュー結果を取得できませんでしたが、APIは応答しました。", nil
	}

	reviewText := string(reviewTextPart)

	if reviewText == "" {
		return "レビュー結果を取得できませんでした。レスポンスのPartが空のテキストです。", nil
	}

	return reviewText, nil
} // 93行目付近
// 最後の } がここにあるべきです！

// EOF
