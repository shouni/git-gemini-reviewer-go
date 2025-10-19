package gemini

import (
	"context"
	"os"
	"testing"
)

// TestNewClient_InvalidAPIKey は、APIキーが空の場合に NewClient がエラーを返すかテストします。
func TestNewClient_InvalidAPIKey(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		APIKey: "",
		// ModelName: "gemini-2.5-flash", // 不要なフィールドを削除
	}

	t.Run("APIキーが空の場合にエラーを返すこと", func(t *testing.T) {
		_, err := NewClient(ctx, cfg)

		if err == nil {
			t.Error("FAIL: APIキーが空の場合、エラーが返されるべきです")
		}

		// 期待されるエラーメッセージの検証
		expectedError := "APIKey is required for Gemini client initialization"
		if err.Error() != expectedError {
			t.Errorf("FAIL: 予期しないエラーメッセージ\n  got: %q\n  want: %q", err.Error(), expectedError)
		}
	})
}

// TestNewClientFromEnv_MissingKey は、環境変数がない場合に NewClientFromEnv がエラーを返すかテストします。
func TestNewClientFromEnv_MissingKey(t *testing.T) {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 重要な処理: 環境変数テストのためのセットアップとクリーンアップ
	originalKey := os.Getenv("GEMINI_API_KEY")
	originalGoogleKey := os.Getenv("GOOGLE_API_KEY")

	// テストのために一時的にキーを解除
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_API_KEY")

	defer func() {
		// テスト後に元の環境変数に戻す
		if originalKey != "" {
			os.Setenv("GEMINI_API_KEY", originalKey)
		}
		if originalGoogleKey != "" {
			os.Setenv("GOOGLE_API_KEY", originalGoogleKey)
		}
	}()
	// -------------------------------------------------------------------------

	t.Run("環境変数がない場合にエラーを返すこと", func(t *testing.T) {
		_, err := NewClientFromEnv(ctx)

		if err == nil {
			t.Error("FAIL: 環境変数がない場合、エラーが返されるべきです")
		}

		// 期待されるエラーメッセージの検証 (client.go のロジックに合わせて修正)
		expectedError := "GEMINI_API_KEY or GOOGLE_API_KEY environment variable is not set"
		if err.Error() != expectedError {
			t.Errorf("FAIL: 予期しないエラーメッセージ\n  got: %q\n  want: %q", err.Error(), expectedError)
		}
	})
}

// TestNewClientFromEnv_Success は、環境変数がある場合にクライアントが成功裏に初期化されるかテストします。
func TestNewClientFromEnv_Success(t *testing.T) {
	ctx := context.Background()

	// -------------------------------------------------------------------------
	// 重要な処理: 環境変数テストのためのセットアップとクリーンアップ
	originalKey := os.Getenv("GEMINI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "DUMMY_API_KEY_FOR_TEST") // ダミーキーを設定
	defer func() {
		// テスト後に元の環境変数に戻す
		if originalKey != "" {
			os.Setenv("GEMINI_API_KEY", originalKey)
		} else {
			os.Unsetenv("GEMINI_API_KEY")
		}
	}()
	// -------------------------------------------------------------------------

	t.Run("環境変数がある場合に成功すること", func(t *testing.T) {
		client, err := NewClientFromEnv(ctx)

		if err != nil {
			t.Fatalf("FAIL: 環境変数が設定されているにも関わらず初期化に失敗しました: %v", err)
		}

		if client == nil {
			t.Fatal("FAIL: クライアントオブジェクトが nil です")
		}
	})
}
