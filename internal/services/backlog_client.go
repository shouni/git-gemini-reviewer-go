package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// BacklogClient はBacklog APIとの通信を管理します。
type BacklogClient struct {
	baseURL string // 例: https://your-space.backlog.jp/api/v2
	apiKey  string
}

// NewBacklogClient はBacklogClientを初期化します。
func NewBacklogClient() (*BacklogClient, error) {
	apiKey := os.Getenv("BACKLOG_API_KEY")
	spaceURL := os.Getenv("BACKLOG_SPACE_URL")

	if apiKey == "" || spaceURL == "" {
		return nil, fmt.Errorf("BACKLOG_API_KEY and BACKLOG_SPACE_URL environment variables must be set for Backlog mode")
	}

	// APIのベースURLを構築
	apiURL := fmt.Sprintf("%s/api/v2", spaceURL)
	return &BacklogClient{
		baseURL: apiURL,
		apiKey:  apiKey,
	}, nil
}

// PostComment は指定された課題IDにコメントを投稿します。
func (c *BacklogClient) PostComment(issueID string, content string) error {
	// 1. APIのエンドポイントを構築
	// issueID は課題キー (例: PROJECT-123) または課題ID (数値) のどちらでも可
	endpoint := fmt.Sprintf("/issues/%s/comments?apiKey=%s", issueID, c.apiKey)
	fullURL := c.baseURL + endpoint

	// 2. リクエストボディを作成 (JSON)
	commentData := map[string]string{
		"content": content,
	}
	jsonBody, err := json.Marshal(commentData)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %w", err)
	}

	// 3. HTTP POST リクエストの実行
	resp, err := http.Post(fullURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to send POST request to Backlog: %w", err)
	}
	defer resp.Body.Close()

	// 4. ステータスコードのチェック
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		// エラーレスポンスボディを詳細に読み取るとより親切ですが、ここではステータスコードで判断
		return fmt.Errorf("Backlog API returned status code %d for issue %s", resp.StatusCode, issueID)
	}

	fmt.Printf("✅ Backlog issue %s successfully commented.\n", issueID)
	return nil
}
