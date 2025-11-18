package prompts

import (
	"fmt"
	"strings"
	"text/template"
)

// ReviewPromptBuilder は、レビューモードとデータに基づいて最終的なAIプロンプトを構築する契約です。
type ReviewPromptBuilder interface {
	Build(mode string, data TemplateData) (string, error)
}

// PromptBuilder はレビュープロンプトの構成を管理し、モード選択のロジックを内包します。
type PromptBuilder struct {
	templates map[string]*template.Template
}

// NewPromptBuilder は PromptBuilder を初期化します。
func NewPromptBuilder() (*PromptBuilder, error) {
	parsedTemplates := make(map[string]*template.Template)
	for mode, content := range allTemplates {
		if content == "" {
			return nil, fmt.Errorf("プロンプトテンプレート '%s' (go:embed) の読み込みに失敗しました: 内容が空です", mode)
		}

		tmpl, err := template.New(mode).Parse(content)
		if err != nil {
			return nil, fmt.Errorf("プロンプト '%s' の解析に失敗: %w", mode, err)
		}
		parsedTemplates[mode] = tmpl
	}

	return &PromptBuilder{
		templates: parsedTemplates,
	}, nil
}

// Build は、要求されたモードに応じて適切なテンプレートを実行します。
func (b *PromptBuilder) Build(mode string, data TemplateData) (string, error) {
	tmpl, ok := b.templates[mode]
	if !ok {
		return "", fmt.Errorf("不明なモードです: '%s'", mode)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("プロンプトテンプレートの実行に失敗しました: %w", err)
	}

	return sb.String(), nil
}
