package geminiclient

import (
	"context"
	"fmt"
	"os"

	"github.com/shouni/go-ai-client/v2/pkg/ai/gemini"
)

const (
	// コードレビューの一貫性を優先するため、低い温度に設定
	defaultGeminiTemperature = float32(0.2)
	// 一時的なネットワークエラーやAPIのレート制限に対応するためのリトライ回数
	defaultGeminiMaxRetries = uint64(3)
)

// Service は、Gemini AIとの通信機能の抽象化を提供し、DIで使用されます。
type Service interface {
	// ReviewCodeDiff は完成されたプロンプトを基にGeminiにレビューを依頼します。
	ReviewCodeDiff(ctx context.Context, finalPrompt string) (string, error)
}

// Client は go-ai-client の gemini.Client をラップし、
// Service インターフェースを実装する具体的な構造体です。
type Client struct {
	// 汎用的な gemini.Client を組み込む
	client    *gemini.Client
	modelName string
}

// NewClient はClientを初期化し、Serviceインターフェースとして返します。
// 温度 0.2 を明示的に指定するため、gemini.NewClientFromEnv ではなく gemini.NewClient を直接利用します。
// APIキーは環境変数から取得し、リトライ回数はデフォルトの3回を設定します。
func NewClient(ctx context.Context, modelName string) (Service, error) {

	// 1. APIキーを環境変数から取得
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY environment variable is not set")
	}

	// 2. モデルパラメータとリトライ設定を定義
	temperature := defaultGeminiTemperature
	maxRetries := defaultGeminiMaxRetries

	// 3. gemini.Config 構造体を構築
	cfg := gemini.Config{
		APIKey:      apiKey,
		Temperature: &temperature,
		MaxRetries:  maxRetries,
	}

	// 4. gemini.NewClient を利用してクライアントを生成
	gClient, err := gemini.NewClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize underlying gemini client: %w", err)
	}

	// Client構造体のインスタンスを Serviceインターフェースとして返す
	return &Client{
		client:    gClient,
		modelName: modelName,
	}, nil
}

// ReviewCodeDiff は Service インターフェースを満たします。
func (c *Client) ReviewCodeDiff(ctx context.Context, finalPrompt string) (string, error) {
	// 汎用クライアントの GenerateContent メソッドを呼び出す
	resp, err := c.client.GenerateContent(ctx, finalPrompt, c.modelName)

	if err != nil {
		// リトライ上限到達などのエラーを含む
		return "", fmt.Errorf("Gemini code review failed: %w", err)
	}

	// 成功レスポンスからテキストを返す
	return resp.Text, nil
}
