package adapters

import (
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

// GCSUploader は、GCSへのデータアップロード操作を抽象化するインターフェースです。
type GCSUploader interface {
	// Upload は、指定されたバケットとパスにコンテンツをアップロードします。
	Upload(ctx context.Context, bucketName string, objectPath string, content string) error
}

// GCSClient は GCSUploader インターフェースの具体的な実装構造体です。
type GCSClient struct {
	client *storage.Client // GCS SDK クライアント
}

// NewGCSUploader は GCSUploader の新しいインスタンスを作成します。
func NewGCSUploader(ctx context.Context) (GCSUploader, error) {
	// Cloud Run環境では、サービスアカウントの認証情報が自動的に使用されます。
	// ローカル開発用に認証オプションを追加することもできます。
	client, err := storage.NewClient(ctx, option.WithTimeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("GCSクライアントの初期化に失敗: %w", err)
	}
	return &GCSClient{client: client}, nil
}

// Upload は、コンテンツを GCS にアップロードします。
func (c *GCSClient) Upload(ctx context.Context, bucketName string, objectPath string, content string) error {
	// 1. GCSのオブジェクトライターを作成
	wc := c.client.Bucket(bucketName).Object(objectPath).NewWriter(ctx)

	// 公開アクセスURLで参照されるため、Content-TypeはHTMLに設定
	wc.ContentType = "text/html"
	// キャッシュ制御: 結果は頻繁に変わらないため、短めのキャッシュを設定
	wc.CacheControl = "public, max-age=300"

	// 2. コンテンツを書き込み
	if _, err := io.WriteString(wc, content); err != nil {
		return fmt.Errorf("GCSへのデータ書き込み中に失敗: %w", err)
	}

	// 3. ライターをクローズしてアップロードを確定
	if err := wc.Close(); err != nil {
		return fmt.Errorf("GCSアップロードのクローズに失敗: %w", err)
	}

	return nil
}
