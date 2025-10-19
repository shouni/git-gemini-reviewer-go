// internal/services/prompt_builder.go

package services

import (
	"fmt"
)

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
	if diffContent == "" {
		// 差分がない場合もエラーとせず、空の差分として処理させることが多いため、エラーにするか検討
		// 今回はエラーとせず、テンプレートだけを返すことで、AIに「差分なし」と伝える
	}

	// テンプレートに diffContent を埋め込む
	prompt := fmt.Sprintf(b.promptTemplate, diffContent)
	return prompt, nil
}
