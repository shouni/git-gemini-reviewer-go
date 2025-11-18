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

// BuildGitService は、アプリケーションの設定に基づいて adapters.GitService の実装を構築します。
func BuildGitService(cfg config.ReviewConfig) adapters.GitService {
	// 1. GitAdapter Optionの設定
	skipHostKeyCheckOption := adapters.WithInsecureSkipHostKeyCheck(cfg.SkipHostKeyCheck)
	baseBranchOption := adapters.WithBaseBranch(cfg.BaseBranch)

	// 2. adapters.NewGitAdapter を呼び出してインスタンスを構築
	gitAdapter := adapters.NewGitAdapter(
		cfg.LocalPath,
		cfg.SSHKeyPath,
		skipHostKeyCheckOption,
		baseBranchOption,
	)

	slog.Debug("GitService (Adapter) を構築しました。",
		slog.String("local_path", cfg.LocalPath),
		slog.String("base_branch", cfg.BaseBranch),
	)

	return gitAdapter
}

// BuildGeminiService は、アプリケーションの設定に基づいて adapters.CodeReviewAI の実装を構築します。
// NewGeminiAdapter は context.Context を必要とするため、引数に追加します。
func BuildGeminiService(ctx context.Context, cfg config.ReviewConfig) (adapters.CodeReviewAI, error) {
	// adapters.NewGeminiAdapter を呼び出してインスタンスを構築
	geminiAdapter, err := adapters.NewGeminiAdapter(ctx, cfg.GeminiModel)
	if err != nil {
		// クライアント構築時のエラーを呼び出し元に返す
		return nil, err
	}

	slog.Debug("GeminiService (Adapter) を構築しました。",
		slog.String("model", cfg.GeminiModel),
	)

	// adapters.CodeReviewAI インターフェースとして返却
	return geminiAdapter, nil
}

// BuildReviewPromptBuilder は、レビューの種類に応じて適切な ReviewPromptBuilder を構築します。
func BuildReviewPromptBuilder() (prompts.ReviewPromptBuilder, error) {
	builder, err := prompts.NewPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("レビュープロンプトビルダーの初期化エラー: %w", err)
	}

	return builder, nil
}

// BuildReviewRunner は、必要な依存関係をすべて構築し、
// 実行可能な ReviewRunner のインスタンスを返します。
func BuildReviewRunner(ctx context.Context, cfg config.ReviewConfig) (*runner.ReviewRunner, error) {
	// 1. GitService の構築
	gitService := BuildGitService(cfg)

	// 2. GeminiService の構築
	geminiService, err := BuildGeminiService(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// 3. Prompt Builder の構築
	promptBuilder, err := BuildReviewPromptBuilder()
	if err != nil {
		return nil, fmt.Errorf("Prompt Builder の構築に失敗しました: %w", err)
	}

	// 4. 依存関係を注入して Runner を組み立てる
	reviewRunner := runner.NewReviewRunner(
		gitService,
		geminiService,
		promptBuilder,
	)

	slog.Debug("ReviewRunner の構築が完了しました。")
	return reviewRunner, nil
}
