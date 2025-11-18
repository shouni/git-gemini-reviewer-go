package adapters

import (
	"context"
	"fmt"
	"io"

	"github.com/shouni/go-text-format/pkg/builder"
	"github.com/shouni/go-text-format/pkg/md2htmlrunner"
)

const ReviewTitle = "AIコードレビュー結果"

// MarkdownToHtmlRunner は、Markdown コンテンツを完全な HTML ドキュメントに変換する契約です。
// Runner の ConvertMarkdownToHtml のシグネチャに合わせます。
type MarkdownToHtmlRunner interface {
	// Run は Markdown コンテンツをバイトスライスで受け取り、HTML コンテンツを含む io.Reader を返します。
	Run(ctx context.Context, markdownContent []byte) (io.Reader, error)
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
func (a *MarkdownConverterAdapter) Run(ctx context.Context, markdownContent []byte) (io.Reader, error) {
	buffer, err := a.coreRunner.ConvertMarkdownToHtml(ctx, ReviewTitle, markdownContent)
	if err != nil {
		return nil, fmt.Errorf("MarkdownからHTMLへの変換に失敗: %w", err)
	}

	return buffer, nil
}
