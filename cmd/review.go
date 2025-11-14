package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"git-gemini-reviewer-go/internal/builder"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/pipeline"
)

// executeReviewPipeline は、すべての依存関係を構築し、レビューパイプラインを実行します。
// 実行結果の文字列とエラーを返します。
func executeReviewPipeline(
	ctx context.Context,
	cfg config.ReviewConfig,
) (string, error) {

	// --- 1. 依存関係の構築（Builder パッケージを使用） ---
	gitService := builder.BuildGitService(cfg)

	geminiService, err := builder.BuildGeminiService(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// promptBuilder の構築
	// cfg.ReviewMode に基づいて適切なテンプレートを選択し、ビルダーを初期化します。
	promptBuilder, err := builder.BuildReviewPromptBuilder(cfg)
	if err != nil {
		return "", fmt.Errorf("Prompt Builder の構築に失敗しました: %w", err)
	}

	slog.Info("レビューパイプラインを開始します。")

	// --- 2. 共通ロジック (Pipeline) の実行 ---
	reviewResult, err := pipeline.RunReviewAndGetResult(
		ctx,
		cfg,
		gitService,
		geminiService,
		promptBuilder,
	)
	if err != nil {
		return "", err
	}

	// --- 3. 結果の返却 ---
	if reviewResult == "" {
		slog.Info("Diff がないためレビューをスキップしました。")
		return "", nil
	}

	return reviewResult, nil
}
