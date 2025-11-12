package prompts

import (
	_ "embed"
	"fmt"
)

// --- テンプレートのリソース定義 (go:embed) ---

//go:embed prompt_release.md
var ReleasePromptTemplate string

//go:embed prompt_detail.md
var DetailPromptTemplate string

// GetReviewTemplate は、レビューモードに基づいて、テンプレート名とその内容を返します。
// エラーは、無効なモードが指定された場合に返されます。
func GetReviewTemplate(reviewMode string) (name string, content string, err error) {
	switch reviewMode {
	case "release":
		name = "release_review"
		content = ReleasePromptTemplate
	case "detail":
		name = "detail_review"
		content = DetailPromptTemplate
	default:
		// builderの堅牢性を高めるためにエラーを返す
		return "", "", fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
	}

	// テンプレートの内容が空でないか（go:embedが失敗していないか）の基本的なチェックも追加できます
	if content == "" {
		return "", "", fmt.Errorf("レビューモード '%s' に対応するプロンプトテンプレートの内容が空です。", reviewMode)
	}

	return name, content, nil
}
