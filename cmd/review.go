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
	logger *slog.Logger,
) (string, error) {

	// --- 1. 依存関係の構築（Builder パッケージを使用） ---
	gitService := builder.BuildGitService(cfg, logger)
	geminiService, err := builder.BuildGeminiService(ctx, cfg, logger)
	if err != nil {
		return "", fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// --- 2. 共通ロジック (Pipeline) の実行 ---

	// 依存サービスを注入して RunReviewAndGetResult を呼び出す
	reviewResult, err := pipeline.RunReviewAndGetResult(
		ctx,
		cfg,
		gitService,
		geminiService,
	)
	if err != nil {
		return "", err
	}

	// --- 3. 結果の返却 ---
	if reviewResult == "" {
		logger.Info("Diff がないためレビューをスキップしました。")
		return "", nil
	}

	return reviewResult, nil
}

// printReviewResult は noPost 時に結果を標準出力します。
func printReviewResult(result string) {
	// 標準出力 (fmt.Println) は維持
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
