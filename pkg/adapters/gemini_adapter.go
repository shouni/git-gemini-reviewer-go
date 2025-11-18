package adapters

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

// CodeReviewAI は、Gemini AIとの通信機能の抽象化を提供し、DIで使用されます。
type CodeReviewAI interface {
	// ReviewCodeDiff は完成されたプロンプトを基にGeminiにレビューを依頼します。
	ReviewCodeDiff(ctx context.Context, finalPrompt string) (string, error)
}

// GeminiAdapter は go-ai-client の gemini.Client をラップし、
// CodeReviewAI インターフェースを実装する具体的な構造体です。
type GeminiAdapter struct {
	client    *gemini.Client
	modelName string
}

// NewGeminiAdapter はGeminiAdapterを初期化し、CodeReviewAIインターフェースとして返します。
// 温度 0.2 を明示的に指定するため、gemini.NewClientFromEnv ではなく gemini.NewClient を直接利用します。
// APIキーは環境変数から取得し、リトライ回数はデフォルトの3回を設定します。
func NewGeminiAdapter(ctx context.Context, modelName string) (CodeReviewAI, error) {

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

	// GeminiAdapter構造体のインスタンスを CodeReviewAIインターフェースとして返す
	return &GeminiAdapter{
		client:    gClient,
		modelName: modelName,
	}, nil
}

// ReviewCodeDiff は CodeReviewAI インターフェースを満たします。
func (ga *GeminiAdapter) ReviewCodeDiff(ctx context.Context, finalPrompt string) (string, error) {
	// 汎用クライアントの GenerateContent メソッドを呼び出す
	resp, err := ga.client.GenerateContent(ctx, finalPrompt, ga.modelName)

	if err != nil {
		return "", fmt.Errorf("Gemini API call failed (Model: %s): %w", ga.modelName, err)
	}

	// 成功レスポンスからテキストを返す
	return resp.Text, nil
}
