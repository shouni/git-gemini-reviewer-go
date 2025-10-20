package cmd

import (
	"fmt"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"
)

// CreateReviewConfig は、コマンドライン引数と選択されたレビューモードに基づき
// config.ReviewConfig 構造体を構築します。
//
// CreateReviewConfigParams は cmd/root.go で定義されています。
func CreateReviewConfig(params CreateReviewConfigParams) (config.ReviewConfig, error) {
	// 1. レビューモードに基づいたプロンプトの選択
	var selectedPrompt string

	switch params.ReviewMode {
	case "release":
		// prompts パッケージからテンプレートを取得
		selectedPrompt = prompts.ReleasePromptTemplate
		fmt.Println("✅ リリースレビューモードが選択されました。")
	case "detail":
		selectedPrompt = prompts.DetailPromptTemplate
		fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
	default:
		return config.ReviewConfig{}, fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", params.ReviewMode)
	}

	// 2. 共通ロジックのための設定構造体を作成
	// params 構造体から値を取得する
	cfg := config.ReviewConfig{
		GeminiModel:      params.GeminiModel,
		PromptContent:    selectedPrompt,
		GitCloneURL:      params.GitCloneURL,
		BaseBranch:       params.BaseBranch,
		FeatureBranch:    params.FeatureBranch,
		SSHKeyPath:       params.SSHKeyPath,
		LocalPath:        params.LocalPath,
		SkipHostKeyCheck: params.SkipHostKeyCheck,
	}

	return cfg, nil
}
