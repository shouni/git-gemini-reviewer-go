package builder

import (
	"context"
	"fmt"
	"log/slog"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/runner"
	"git-gemini-reviewer-go/pkg/adapters"
	"git-gemini-reviewer-go/pkg/prompts"
)

// BuildReviewRunner は、必要な依存関係をすべて構築し、
// 実行可能な ReviewRunner のインスタンスを返します。
func BuildReviewRunner(ctx context.Context, cfg config.ReviewConfig) (*runner.ReviewRunner, error) {
	// 1. GitService の構築
	gitService := adapters.NewGitAdapter(
		cfg.LocalPath,
		cfg.SSHKeyPath,
		adapters.WithInsecureSkipHostKeyCheck(cfg.SkipHostKeyCheck),
		adapters.WithBaseBranch(cfg.BaseBranch),
	)
	slog.Debug("GitService (Adapter) を構築しました。",
		slog.String("local_path", cfg.LocalPath),
		slog.String("base_branch", cfg.BaseBranch),
	)

	// 2. GeminiService の構築
	geminiService, err := adapters.NewGeminiAdapter(ctx, cfg.GeminiModel)
	if err != nil {
		return nil, fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}
	slog.Debug("GeminiService (Adapter) を構築しました。", "model", cfg.GeminiModel)

	// 3. Prompt Builder の構築
	promptBuilder, err := prompts.NewPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("Prompt Builder の構築に失敗しました: %w", err)
	}
	slog.Debug("PromptBuilderを構築しました。")

	// 4. 依存関係を注入して Runner を組み立てる
	reviewRunner := runner.NewReviewRunner(
		gitService,
		geminiService,
		promptBuilder,
	)

	slog.Debug("ReviewRunner の構築が完了しました。")
	return reviewRunner, nil
}
