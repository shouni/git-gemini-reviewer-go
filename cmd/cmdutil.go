package cmd

import (
	"fmt"
	"git-gemini-reviewer-go/internal/services"
	"git-gemini-reviewer-go/prompts"
)

// CreateReviewConfig は、グローバルフラグと選択されたレビューモードに基づき
// services.ReviewConfig 構造体を構築します。
// NOTE: この関数は cmd/root.go で定義されたグローバル変数に依存します。
func CreateReviewConfig() (services.ReviewConfig, error) {
	// 1. レビューモードに基づいたプロンプトの選択
	var selectedPrompt string

	// reviewMode は cmd/root.go の Persistent Flag の変数を使用
	switch reviewMode {
	case "release":
		// services パッケージからテンプレートを取得
		selectedPrompt = prompts.ReleasePromptTemplate
		fmt.Println("✅ リリースレビューモードが選択されました。")
	case "detail":
		selectedPrompt = prompts.DetailPromptTemplate
		fmt.Println("✅ 詳細レビューモードが選択されました。（デフォルト）")
	default:
		return services.ReviewConfig{}, fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", reviewMode)
	}

	// 2. 共通ロジックのための設定構造体を作成
	// すべて cmd/root.go で定義された共通変数を使用
	cfg := services.ReviewConfig{
		GeminiModel:      geminiModel,
		PromptContent:    selectedPrompt,
		GitCloneURL:      gitCloneURL,
		BaseBranch:       baseBranch,
		FeatureBranch:    featureBranch,
		SSHKeyPath:       sshKeyPath,
		LocalPath:        localPath,
		SkipHostKeyCheck: skipHostKeyCheck,
	}

	return cfg, nil
}
