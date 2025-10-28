package services

import (
	"context"
	"fmt"
	"os"

	"github.com/shouni/go-ai-client/pkg/ai/gemini"
)

// GeminiClient は go-ai-client の gemini.Client をラップし、
// git-gemini-reviewer-go のサービス層向けインターフェースを提供します。
type GeminiClient struct {
	// 汎用的な gemini.Client を組み込む
	client    *gemini.Client
	modelName string
}

// NewGeminiClient はGeminiClientを初期化します。
// 温度 0.2 を明示的に指定し、APIキーは環境変数から取得します。
func NewGeminiClient(ctx context.Context, modelName string) (*GeminiClient, error) {

	// 1. APIキーを環境変数から取得 (gemini.NewClientFromEnv のロジックを部分的に移植)
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY environment variable is not set")
	}

	// 2. 温度 0.2 を設定
	// *float32 型が必要なため、float32 の変数を作成し、そのポインタを渡す
	temperature := float32(0.2)

	// 3. gemini.Config 構造体を構築
	cfg := gemini.Config{
		APIKey: apiKey,
		// Temperatureフィールドに、作成したポインタを渡す
		Temperature: &temperature,
		// MaxRetriesは設定しないため、デフォルト値が使用される
	}

	// 4. gemini.NewClient を利用してクライアントを生成
	gClient, err := gemini.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize underlying gemini client: %w", err)
	}

	return &GeminiClient{
		client:    gClient,
		modelName: modelName,
	}, nil
}

// ReviewCodeDiff は完成されたプロンプトを基にGeminiにレビューを依頼します。
// リトライ処理は gemini.Client.GenerateContent に内蔵されているため、ここでは単に呼び出すだけです。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, finalPrompt string) (string, error) {

	// 汎用クライアントの GenerateContent メソッドを呼び出す
	resp, err := c.client.GenerateContent(ctx, finalPrompt, c.modelName)

	if err != nil {
		// リトライ上限到達などのエラーを含む
		return "", fmt.Errorf("Gemini code review failed: %w", err)
	}

	// 成功レスポンスからテキストを返す
	return resp.Text, nil
}
