package services

import (
	"context"
	"fmt"

	"git-gemini-reviewer-go/internal/pkg/gemini"
)

// GeminiClient は go-ai-client の gemini.Client をラップし、
// git-gemini-reviewer-go のサービス層向けインターフェースを提供します。
type GeminiClient struct {
	// 汎用的な gemini.Client を組み込む
	client    *gemini.Client
	modelName string
}

// NewGeminiClient はGeminiClientを初期化します。
// NewClientFromEnv を利用することで、APIキーの取得とリトライ設定の初期化は gemini パッケージに任せます。
func NewGeminiClient(ctx context.Context, modelName string) (*GeminiClient, error) {

	// gemini.NewClientFromEnv を利用し、APIキーとデフォルトリトライ設定を持つクライアントを生成
	gClient, err := gemini.NewClientFromEnv(ctx)
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
