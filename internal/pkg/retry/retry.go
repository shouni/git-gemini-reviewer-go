package retry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
)

const (
	// リトライ関連の定数
	DefaultMaxRetries = 3 // 最大リトライ回数

	// バックオフのカスタム設定
	InitialBackoffInterval = 500 * time.Millisecond
	MaxBackoffInterval     = 5 * time.Second
)

// Operation はリトライ可能な処理を表す関数です。成功時は nil を返します。
type Operation func() error

// ShouldRetryFunc はエラーを受け取り、そのエラーがリトライ可能かどうかを判定する関数です。
type ShouldRetryFunc func(error) bool

// Config はリトライ動作を設定するための構造体です。
type Config struct {
	MaxRetries      uint64
	InitialInterval time.Duration
	MaxInterval     time.Duration
}

// DefaultConfig は推奨されるデフォルト設定を返します。
func DefaultConfig() Config {
	return Config{
		MaxRetries:      DefaultMaxRetries,
		InitialInterval: InitialBackoffInterval,
		MaxInterval:     MaxBackoffInterval,
	}
}

// Do は指数バックオフとカスタムエラー判定を使用して操作をリトライします。
// Configを引数で受け取ることで、特定のクライアント構造体（Client）への依存を排除しています。
func Do(ctx context.Context, cfg Config, operationName string, op Operation, shouldRetryFn ShouldRetryFunc) error {

	// backoff の設定
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = cfg.InitialInterval
	b.MaxInterval = cfg.MaxInterval

	// 最大リトライ回数とコンテキストを backoff に適用
	bo := backoff.WithMaxRetries(b, cfg.MaxRetries)
	bo = backoff.WithContext(bo, ctx)

	var lastErr error

	// リトライ処理内で実行される実際の操作
	retryableOp := func() error {
		err := op()

		if err == nil {
			return nil // 成功
		}

		// 外部から渡された判定関数を使用
		if shouldRetryFn(err) {
			lastErr = fmt.Errorf("一時的なエラーが発生、リトライします: %w", err)
			return lastErr // リトライ対象
		}

		lastErr = fmt.Errorf("致命的なエラーのためリトライを中止: %w", err)
		return backoff.Permanent(lastErr) // 永続エラーとしてラップし、即時終了
	}

	err := backoff.Retry(retryableOp, bo)

	if err != nil {
		// コンテキストキャンセル/タイムアウトのエラー処理
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return fmt.Errorf("%sに失敗しました: コンテキストタイムアウト/キャンセル: %w", operationName, err)
		}

		// backoff.Permanent でラップされたエラーから元のエラーを取得
		if pErr, ok := err.(*backoff.PermanentError); ok {
			return pErr.Err // 最後の致命的なエラーを返す
		}

		// その他のリトライ上限到達エラー
		return fmt.Errorf("%sに失敗しました: 最大リトライ回数 (%d回) に到達。最終エラー: %w", operationName, cfg.MaxRetries, lastErr)
	}
	return nil
}
