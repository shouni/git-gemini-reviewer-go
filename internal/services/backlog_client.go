package services

import (
	"bytes"
	"context" // contextをインポート
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	// 移植した内部リトライパッケージをインポート
	"git-gemini-reviewer-go/internal/pkg/retry"
	// backoff.Permanent を使用するためにインポート
	"github.com/cenkalti/backoff/v4"
)

// BacklogClient はBacklog APIとの通信を管理します。
type BacklogClient struct {
	client      *http.Client
	baseURL     string // 例: https://your-space.backlog.jp/api/v2
	apiKey      string
	retryConfig retry.Config // リトライ設定を追加
}

// BacklogErrorResponse はBacklog APIが返す一般的なエラー構造体です。
type BacklogErrorResponse struct {
	Errors []struct {
		Message string `json:"message"`
		Code    int    `json://code"`
	} `json:"errors"`
}

// 絵文字をマッチさせるための正規表現パターン。
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
		// デフォルトのリトライ設定を初期化
		retryConfig: retry.DefaultConfig(),
	}, nil
}

// PostComment は指定された課題IDにコメントを投稿します。
// 外部から呼ばれるメインの投稿関数で、リトライ機構を組み込みます。
func (c *BacklogClient) PostComment(ctx context.Context, issueID string, content string) error {

	// --- 投稿操作を定義する関数 ---
	op := func() error {
		return c.postCommentAttempt(issueID, content)
	}

	// --- エラー判定と再試行ロジックの定義 ---
	shouldRetryFn := func(err error) bool {
		var backlogErr *BacklogError

		if errors.As(err, &backlogErr) {
			// Backlog APIが4xxクライアントエラーを返した場合
			if backlogErr.StatusCode >= 400 && backlogErr.StatusCode < 500 {
				// 永続的なエラーとして扱う (例: 認証失敗、不正な課題ID)
				return false
			}
			// 5xxサーバーエラーは一時的なものとしてリトライ
			if backlogErr.StatusCode >= 500 {
				return true
			}
		}

		// ネットワークエラーなどはリトライ対象
		return true
	}

	// --- リトライの実行 ---
	err := retry.Do(
		ctx,
		c.retryConfig,
		fmt.Sprintf("Backlog comment post for issue %s", issueID),
		op,
		shouldRetryFn,
	)

	if err != nil {
		// リトライ上限に達した、または永続エラーとして終了した場合

		// 永続エラーの確認
		var pErr *backoff.PermanentError
		if errors.As(err, &pErr) {
			// 永続エラーの場合、絵文字が原因であるか確認し、サニタイズして再試行
			if strings.Contains(pErr.Err.Error(), "Incorrect String") {
				fmt.Printf("⚠️ Backlog API returned 'Incorrect String' error. Sanitizing comment and trying once more...\n")

				sanitizedContent := cleanStringFromEmojis(content)
				if sanitizedContent == content {
					return fmt.Errorf("failed to post comment: %w (no fixable content issues)", pErr.Err)
				}

				// サニタイズ後の最終試行 (リトライなし)
				retryErr := c.postCommentAttempt(issueID, sanitizedContent)
				if retryErr == nil {
					fmt.Printf("✅ Backlog issue %s successfully commented after sanitizing.\n", issueID)
					return nil
				}
				return fmt.Errorf("failed to post comment after sanitizing for issue %s: %w", issueID, retryErr)
			}
		}

		// その他のエラー
		return fmt.Errorf("failed to post comment to Backlog API for issue %s after retries: %w", issueID, err)
	}

	fmt.Printf("✅ Backlog issue %s successfully commented.\n", issueID)
	return nil
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
		return backoff.Permanent(fmt.Errorf("failed to marshal comment data: %w", err)) // 永続エラー
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return backoff.Permanent(fmt.Errorf("failed to create POST request for Backlog: %w", err)) // 永続エラー
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		// ネットワークエラーは通常のエラーとして返し、リトライ対象とする
		return fmt.Errorf("failed to send POST request to Backlog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// エラーレスポンスの処理
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &BacklogError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("failed to read error body: %s", err.Error())}
	}

	var errorResp BacklogErrorResponse
	if json.Unmarshal(body, &errorResp) == nil && len(errorResp.Errors) > 0 {
		firstError := errorResp.Errors[0]
		// 'Incorrect String' (絵文字など) のエラーは Permanent として返す
		if strings.Contains(firstError.Message, "Incorrect String") {
			return backoff.Permanent(&BacklogError{StatusCode: resp.StatusCode, Code: firstError.Code, Message: firstError.Message})
		}
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
