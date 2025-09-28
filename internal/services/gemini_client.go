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
// プロンプトファイルは、コード差分(%s)を埋め込むための Go 標準の fmt.Sprintf 形式のプレースホルダを持っている必要があります。
func (c *GeminiClient) ReviewCodeDiff(ctx context.Context, codeDiff string, promptFilePath string) (string, error) {
	// 1. プロンプトファイルの読み込み
	// ioutil.ReadFile は非推奨なので os.ReadFile に置き換え
	promptTemplateBytes, err := os.ReadFile(promptFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %s: %w", promptFilePath, err)
	}
	promptTemplate := string(promptTemplateBytes)

	// 2. プロンプトの構成
	// プロンプトファイルの内容をテンプレートとして使用し、コード差分を埋め込む
	prompt := fmt.Sprintf(promptTemplate, codeDiff)

	// 3. API呼び出し
	model := c.client.GenerativeModel(c.modelName)
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("GenerateContent failed with model %s: %w", c.modelName, err)
	}

	// 4. レスポンスの処理
	if resp == nil || len(resp.Candidates) == 0 {
		return "", fmt.Errorf("received empty or invalid response from Gemini API")
	}

	candidate := resp.Candidates[0]

	// 応答がブロックされた場合（セキュリティフィルタなど）のチェック
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		// 応答がブロックされた詳細情報を確認
		// 💡 修正: genai.FinishReasonUnspecified と比較
		if candidate.FinishReason != genai.FinishReasonUnspecified {
			// String() メソッドを使用して FinishReason を文字列化
			return "", fmt.Errorf("API response was blocked or finished prematurely. Reason: %s", candidate.FinishReason.String())
		}
		return "", fmt.Errorf("Gemini response candidate is empty or lacks content parts")
	}

	// 5. テキスト内容の抽出
	reviewText, ok := candidate.Content.Parts[0].(genai.Text)
	if !ok {
		// テキスト以外のデータ型が返された場合（予期しないケース）
		return "", fmt.Errorf("API returned non-text part in response")
	}

	result := string(reviewText)
	if result == "" {
		return "", fmt.Errorf("API returned an empty text review result")
	}

	return result, nil
}
