package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// BacklogClient はBacklog APIとの通信を管理します。
type BacklogClient struct {
	client  *http.Client
	baseURL string // 例: https://your-space.backlog.jp/api/v2
	apiKey  string
}

// BacklogErrorResponse はBacklog APIが返す一般的なエラー構造体です。
type BacklogErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"errors"`
}

// 絵文字をマッチさせるための正規表現パターン。
// \x{0080}-\x{FFFF} は基本多言語面と一部のBMP外文字（絵文字が含まれる可能性のある一般的な範囲）をカバーします。
// より厳密な絵文字の範囲は Unicode 標準で定義されていますが、Goのregexpパッケージの制限内で
// 一般的な「表示の問題を起こす可能性のある非ASCII文字」を対象とするアプローチです。
// あるいは、`\p{Emoji_Presentation}` (Go 1.18+) を試すこともできますが、環境によっては動かない場合があるため、
var emojiRegex = regexp.MustCompile(`[^\x00-\x7F]`) // ASCII 以外の文字を全て除去

// cleanStringFromEmojis は、文字列から絵文字を削除します。
func cleanStringFromEmojis(s string) string {
	return emojiRegex.ReplaceAllString(s, "")
}

// NewBacklogClient はBacklogClientを初期化します。
func NewBacklogClient(spaceURL string, apiKey string) (*BacklogClient, error) {
	if spaceURL == "" || apiKey == "" {
		return nil, errors.New("BACKLOG_SPACE_URL および BACKLOG_API_KEY の設定が必要です")
	}

	trimmedURL := strings.TrimRight(spaceURL, "/")
	apiURL := trimmedURL + "/api/v2"

	return &BacklogClient{
		client:  &http.Client{},
		baseURL: apiURL,
		apiKey:  apiKey,
	}, nil
}

// PostComment は指定された課題IDにコメントを投稿します。
// 最初の試行が失敗した場合、絵文字を除去して再試行します。
func (c *BacklogClient) PostComment(issueID string, content string) error {
	// 1. 最初の投稿試行
	err := c.postCommentAttempt(issueID, content)
	if err == nil {
		fmt.Printf("✅ Backlog issue %s successfully commented.\n", issueID)
		return nil
	}

	// 2. エラータイプを判定
	var backlogErr *BacklogError
	if errors.As(err, &backlogErr) {
		// APIエラーであり、かつ「不適切な文字列」エラーであるか確認
		// Backlogのエラーメッセージを正確に確認する必要がありますが、ここでは一般的なチェックを行います
		if backlogErr.StatusCode == http.StatusBadRequest && strings.Contains(backlogErr.Message, "Incorrect String") {
			fmt.Printf("⚠️ Backlog API returned 'Incorrect String' error. Sanitizing comment and retrying...\n")

			// 3. コメントから絵文字を除去
			sanitizedContent := cleanStringFromEmojis(content)

			// 変更がない場合は再試行しない
			if sanitizedContent == content {
				return fmt.Errorf("failed to post comment: %w (no emojis found to remove)", backlogErr)
			}

			// 4. サニタイズ後の再投稿試行
			retryErr := c.postCommentAttempt(issueID, sanitizedContent)
			if retryErr == nil {
				fmt.Printf("✅ Backlog issue %s successfully commented after sanitizing.\n", issueID)
				return nil
			}

			// 再試行でも失敗した場合
			return fmt.Errorf("failed to post comment after sanitizing for issue %s: %w", issueID, retryErr)
		}
	}

	// その他のエラーの場合はそのまま返す
	return fmt.Errorf("failed to post comment to Backlog API for issue %s: %w", issueID, err)
}

// postCommentAttempt はAPIリクエストを実際に実行する内部ヘルパーメソッドです。
func (c *BacklogClient) postCommentAttempt(issueID string, content string) error {
	endpoint := fmt.Sprintf("/issues/%s/comments?apiKey=%s", issueID, c.apiKey) // apiKey をクエリパラメータに追加
	fullURL := c.baseURL + endpoint

	commentData := map[string]string{
		"content": content,
	}
	jsonBody, err := json.Marshal(commentData)
	if err != nil {
		return fmt.Errorf("failed to marshal comment data: %w", err)
	}

	// 注: Backlog APIは通常、apiKeyをクエリパラメータで渡すため、
	// Authorizationヘッダーは不要か、使用されていません。
	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create POST request for Backlog: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send POST request to Backlog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &BacklogError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("failed to read error body: %s", err.Error())}
	}

	var errorResp BacklogErrorResponse
	// json.Unmarshalが失敗しても、bodyにはエラーメッセージが含まれている可能性があるため続行
	if json.Unmarshal(body, &errorResp) == nil && len(errorResp.Errors) > 0 {
		firstError := errorResp.Errors[0]
		return &BacklogError{
			StatusCode: resp.StatusCode,
			Code:       firstError.Code,
			Message:    firstError.Message,
		}
	}

	return &BacklogError{
		StatusCode: resp.StatusCode,
		Message:    string(body),
	}
}

// --- カスタムエラー構造体 ---

// BacklogError はBacklog APIから返されるエラーを表すカスタムエラーです。
type BacklogError struct {
	StatusCode int
	Code       int
	Message    string
}

func (e *BacklogError) Error() string {
	return fmt.Sprintf("Backlog API error (status %d, code %d): %s", e.StatusCode, e.Code, e.Message)
}
