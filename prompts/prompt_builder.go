package prompts

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed release_review_prompt.md
var ReleasePromptTemplate string

//go:embed detail_review_prompt.md
var DetailPromptTemplate string

// ReviewPromptBuilder はレビュープロンプトの構成を管理します。
type ReviewPromptBuilder struct {
	// 差分を埋め込むための fmt.Sprintf 形式のテンプレート文字列を保持します
	promptTemplate string
}

// NewReviewPromptBuilder は ReviewPromptBuilder を初期化します。
func NewReviewPromptBuilder(template string) *ReviewPromptBuilder {
	return &ReviewPromptBuilder{promptTemplate: template}
}

// Build はコード差分を埋め込み、Geminiへ送るための最終的なプロンプト文字列を完成させます。
func (b *ReviewPromptBuilder) Build(diffContent string) (string, error) {
	if b.promptTemplate == "" {
		return "", fmt.Errorf("prompt template is not configured")
	}

	// 1. 【クリティカルな指摘への対応】: テンプレートに %s が必須であることを確認
	// これにより、AIに差分が渡されないバグを防ぎます。
	if !strings.Contains(b.promptTemplate, "%s") {
		return "", fmt.Errorf("prompt template is missing the required %%s placeholder for code diff insertion")
	}

	// 2. 【diffContent が空の場合の意図の維持】
	// diffContent が空の場合でも、意図通り空文字列を %s に埋め込み、AIに「差分なし」の状況を伝えます。
	// このチェックは、呼び出し元 (review_logic.go) ですでに空文字列("")を返してスキップする処理があるため、
	// ここでは特にロジック変更は不要です。fmt.Sprintf がそのまま空文字列を埋め込みます。

	// テンプレートに diffContent を埋め込む
	prompt := fmt.Sprintf(b.promptTemplate, diffContent)
	return prompt, nil
}
