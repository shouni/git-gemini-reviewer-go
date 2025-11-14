package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"git-gemini-reviewer-go/internal/builder"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/pipeline"

	"github.com/shouni/go-utils/urlpath"
)

// executeReviewPipeline は、すべての依存関係を構築し、レビューパイプラインを実行します。
// 実行結果の文字列とエラーを返します。
func executeReviewPipeline(
	ctx context.Context,
	cfg config.ReviewConfig,
) (string, error) {

	// --- 1. ローカルパスの決定 ---
	// LocalPathが指定されていない場合、RepoURLから動的に生成しcfgを更新します。
	if cfg.LocalPath == "" {
		cfg.LocalPath = urlpath.SanitizeURLToUniquePath(cfg.RepoURL)
		slog.Debug("LocalPathが未指定のため、URLから動的にパスを生成しました。", "generatedPath", cfg.LocalPath)
	}

	// --- 2. サービス依存関係の構築 ---
	gitService := builder.BuildGitService(cfg)

	geminiService, err := builder.BuildGeminiService(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// --- 3. Prompt Builder の構築 ---
	// cfg.ReviewMode に基づいて適切なテンプレートを選択し、ビルダーを初期化します。
	promptBuilder, err := builder.BuildReviewPromptBuilder(cfg)
	if err != nil {
		return "", fmt.Errorf("Prompt Builder の構築に失敗しました: %w", err)
	}

	slog.Info("レビューパイプラインを開始します。")

	// --- 4. 共通ロジック (Pipeline) の実行 ---
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

	// --- 5. 結果の返却 ---
	if reviewResult == "" {
		slog.Info("Diff がないためレビューをスキップしました。")
		return "", nil
	}

	return reviewResult, nil
}
