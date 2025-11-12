package prompts

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

// ----------------------------------------------------------------
// テンプレート構造体
// ----------------------------------------------------------------

// ReviewTemplateData はレビュープロンプトのテンプレートに渡すデータ構造です。
type ReviewTemplateData struct {
	DiffContent string
}

// ----------------------------------------------------------------
// ビルダー実装
// ----------------------------------------------------------------

// ReviewPromptBuilder はレビュープロンプトの構成を管理します。
type ReviewPromptBuilder struct {
	// 差分を埋め込むための text/template を保持します
	tmpl *template.Template
}

// NewReviewPromptBuilder は ReviewPromptBuilder を初期化します。
// テンプレート文字列を受け取り、それをパースして *template.Template を保持します。
// name はテンプレートの名前であり、主にデバッグやエラーメッセージの識別に利用されます。
func NewReviewPromptBuilder(name string, templateContent string) (*ReviewPromptBuilder, error) {
	if templateContent == "" {
		return nil, fmt.Errorf("プロンプトテンプレートの内容が空です")
	}

	tmpl, err := template.New(name).Parse(templateContent)
	if err != nil {
		return nil, fmt.Errorf("プロンプトテンプレートの解析に失敗しました: %w", err)
	}
	return &ReviewPromptBuilder{tmpl: tmpl}, nil
}

// Build は ReviewTemplateData を埋め込み、Geminiへ送るための最終的なプロンプト文字列を完成させます。
func (b *ReviewPromptBuilder) Build(data ReviewTemplateData) (string, error) {
	if b.tmpl == nil {
		return "", fmt.Errorf("レビュープロンプトテンプレートが適切に初期化されていません。NewReviewPromptBuilderが正しく呼び出されたか確認してください")
	}

	var sb strings.Builder
	if err := b.tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("レビュープロンプトの実行に失敗しました: %w", err)
	}

	return sb.String(), nil
}
