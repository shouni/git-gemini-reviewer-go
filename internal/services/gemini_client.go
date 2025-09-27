package services

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// ReviewPromptTemplate は、コードレビューのためのプロンプトテンプレートです。
// %s を実際の差分で置き換えます。
const ReviewPromptTemplate = `
Review the code diff (in diff format).

**Output MUST be in Markdown format, following this structure:**
1.  **Summary:** A brief summary of the review.
2.  **File Specific Issues:**
    - For each file, use a level-four heading: #### File Name: path/to/your/file.go
    - List any issues found using a list format (-).
    - Each issue must include: **Line Number**, **Problem Description**, and **Suggested Fix**.
    -  修正案では、具体的なコードを適切な言語のコードブロックで示してください。.
- If no issues are found in a file, state: "No issues found."

> **IMPORTANT: Line numbers must be based on the modified file (indicated by '+' in the diff).**

--- diff start ---
%s
--- diff end ---
`

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
		// エラーメッセージをよりシンプルに
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}

	// Geminiクライアントを初期化
	// 通常、クライアントは再利用されるため、context.Background()を使用
	client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		// エラーチェーンを維持しつつ、より具体的なメッセージ
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
// コンテキストを受け取るように変更し、リクエストのキャンセルやタイムアウトに対応できるようにしました。
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

	// 最初のパートからテキストを抽出し、型アサーションが失敗しないようにstringに変換
	if textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
		return string(textPart), nil
	}

	return "", fmt.Errorf("Gemini response content was not in expected text format")
}
