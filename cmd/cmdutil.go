package cmd

import (
	"fmt"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"
)

// CreateReviewConfig は、フラグからバインドされた設定構造体を受け取り、
// ReviewMode フィールドに基づいて適切なプロンプトテンプレートを設定します。
//
// この関数は設定の構築に専念し、副作用（ログ出力など）を持ちません。
func CreateReviewConfig(baseConfig config.ReviewConfig) (config.ReviewConfig, error) {

	// 呼び出し元でフラグからバインドされた ReviewMode フィールドを参照
	switch baseConfig.ReviewMode {
	case "release":
		baseConfig.PromptContent = prompts.ReleasePromptTemplate

	case "detail":
		// DetailPromptTemplate を設定
		baseConfig.PromptContent = prompts.DetailPromptTemplate

	default:
		// 不明なモードが指定された場合は、エラーを返します
		return config.ReviewConfig{}, fmt.Errorf("無効なレビューモードが指定されました: '%s'。'release' または 'detail' を選択してください。", baseConfig.ReviewMode)
	}

	// 適切な PromptContent が設定された baseConfig を返す
	return baseConfig, nil
}

// printReviewResult は noPost 時に結果を標準出力します。
func printReviewResult(result string) {
	// 標準出力 (fmt.Println) は維持
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
