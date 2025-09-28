package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// BacklogClient はBacklog APIとの通信を管理します。
type BacklogClient struct {
	client  *http.Client
	baseURL string // 例: https://your-space.backlog.jp/api/v2
	apiKey  string
}

// BacklogErrorResponse はBacklog APIが返す一般的なエラー構造体です。
// APIからの詳細なエラーメッセージを抽出するために使用します。
type BacklogErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors"`
}

// NewBacklogClient はBacklogClientを初期化します。
func NewBacklogClient(spaceURL string, apiKey string) (*BacklogClient, error) {

	if spaceURL == "" || apiKey == "" {
		// 💡 errors パッケージを使用
		return nil, errors.New("BACKLOG_SPACE_URL および BACKLOG_API_KEY の設定が必要です")
	}

	// URLの正規化: 末尾の / を取り除き、/api/v2 を追加
	trimmedURL := strings.TrimRight(spaceURL, "/")
	apiURL := trimmedURL + "/api/v2"

	return &BacklogClient{
		client:  &http.Client{}, // デフォルトのHTTPクライアント
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

	// 3. HTTP POST リクエストを作成
	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create POST request for Backlog: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 4. HTTP POST リクエストの実行 (c.client を使用)
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send POST request to Backlog: %w", err)
	}
	defer resp.Body.Close()

	// 5. ステータスコードのチェック
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// 成功ステータス (200 OK または 201 Created)
		fmt.Printf("✅ Backlog issue %s successfully commented. Status: %d\n", issueID, resp.StatusCode)
		return nil
	}

	// 6. エラーレスポンスボディの読み取りと詳細エラーメッセージの生成
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// ボディ読み取り自体が失敗した場合
		return fmt.Errorf("Backlog API returned status %d. Failed to read error body: %w", resp.StatusCode, err)
	}

	// 🚀 Backlog APIがJSON形式のエラーを返すことを期待してパースを試みる
	var errorResp BacklogErrorResponse
	if json.Unmarshal(body, &errorResp) == nil && len(errorResp.Errors) > 0 {
		// パースに成功し、エラーメッセージがある場合
		firstError := errorResp.Errors[0]
		return fmt.Errorf("Backlog API error (status %d, code %d) for issue %s: %s",
			resp.StatusCode, firstError.Code, issueID, firstError.Message)
	}

	// JSONパースに失敗、または予期せぬエラー形式の場合
	return fmt.Errorf("Backlog API returned status %d for issue %s. Response body: %s",
		resp.StatusCode, issueID, string(body))
}
