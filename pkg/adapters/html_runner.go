package adapters

import (
	"context"
	"fmt"

	"github.com/shouni/go-text-format/pkg/builder"
	"github.com/shouni/go-text-format/pkg/md2htmlrunner"
)

// MarkdownToHtmlRunner は、Markdown コンテンツを完全な HTML ドキュメントに変換する契約です。
// Runner の ConvertMarkdownToHtml のシグネチャに合わせます。
type MarkdownToHtmlRunner interface {
	// Run のシグネチャをターゲットパッケージのコアメソッドに合わせます。
	Run(ctx context.Context, markdownContent []byte) (htmlContent string, err error)
}

// MarkdownConverterAdapter は go-text-format のロジックをラップしたアダプターです。
type MarkdownConverterAdapter struct {
	coreRunner *md2htmlrunner.MarkdownToHtmlRunner
}

func NewMarkdownToHtmlRunner(ctx context.Context) (MarkdownToHtmlRunner, error) {
	// (ctx は NewBuilder や BuildMarkdownToHtmlRunner が将来必要とする可能性を考慮し、
	//  現在は使用していなくてもシグネチャを合わせる)

	// 1. go-text-format の Builder を初期化 (依存関係の構築)
	md2htmlBuilder, err := builder.NewBuilder()
	if err != nil {
		return nil, fmt.Errorf("go-text-format builderの初期化に失敗: %w", err)
	}

	// 2. Builder を使用して Runner インスタンスを構築
	coreRunner, err := md2htmlBuilder.BuildMarkdownToHtmlRunner()
	if err != nil {
		return nil, fmt.Errorf("MarkdownToHtmlRunnerの構築に失敗: %w", err)
	}

	return &MarkdownConverterAdapter{
		coreRunner: coreRunner,
	}, nil
}

// Run は MarkdownToHtmlRunner インターフェースを満たします。
func (a *MarkdownConverterAdapter) Run(ctx context.Context, markdownContent []byte) (string, error) {
	// Web Runner のレビュー結果に設定する固定タイトル
	const reviewTitle = "AIコードレビュー結果"

	// ターゲットのコアメソッドを呼び出す
	buffer, err := a.coreRunner.ConvertMarkdownToHtml(ctx, reviewTitle, markdownContent)
	if err != nil {
		return "", fmt.Errorf("MarkdownからHTMLへの変換に失敗: %w", err)
	}

	// bytes.Buffer の内容を文字列として返す
	return buffer.String(), nil
}
