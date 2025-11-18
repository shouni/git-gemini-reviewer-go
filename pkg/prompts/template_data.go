package prompts

import (
	_ "embed"
)

// TemplateData はレビュープロンプトのテンプレートに渡すデータ構造です。
type TemplateData struct {
	DiffContent string
}

var (
	//go:embed prompt_release.md
	releasePromptTemplate string
	//go:embed prompt_detail.md
	detailPromptTemplate string
)

// allTemplates は、テンプレートのMAP
var allTemplates = map[string]string{
	"release": releasePromptTemplate,
	"detail":  detailPromptTemplate,
}
