package builder

import (
	"context"
	"fmt"
	"log/slog"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/geminiclient"
	"git-gemini-reviewer-go/internal/gitclient"
	"git-gemini-reviewer-go/internal/prompts"
)

// 既存の BuildGitService 関数を修正して依存パスを調整
// BuildGitService は、アプリケーションの設定に基づいて gitclient.Service の実装を構築します。
func BuildGitService(cfg config.ReviewConfig) gitclient.Service {
	//

	// 1. GitClientOptionの設定
	skipHostKeyCheckOption := gitclient.WithInsecureSkipHostKeyCheck(cfg.SkipHostKeyCheck)
	baseBranchOption := gitclient.WithBaseBranch(cfg.BaseBranch)

	// 2. gitclient.NewClient を呼び出してインスタンスを構築
	gitClient := gitclient.NewClient(
		cfg.LocalPath,
		cfg.SSHKeyPath,
		skipHostKeyCheckOption,
		baseBranchOption,
	)

	slog.Debug("GitServiceを構築しました。",
		slog.String("local_path", cfg.LocalPath),
		slog.String("base_branch", cfg.BaseBranch),
	)

	return gitClient
}

// BuildGeminiService は、アプリケーションの設定に基づいて geminiclient.Service の実装を構築します。
// NewClient は context.Context を必要とするため、引数に追加します。
func BuildGeminiService(ctx context.Context, cfg config.ReviewConfig) (geminiclient.Service, error) {

	// geminiclient.NewClient を呼び出してインスタンスを構築
	geminiClient, err := geminiclient.NewClient(ctx, cfg.GeminiModel)
	if err != nil {
		// クライアント構築時のエラーを呼び出し元に返す
		return nil, err
	}

	slog.Debug("GeminiServiceを構築しました。",
		slog.String("model", cfg.GeminiModel),
	)

	// geminiclient.Serviceインターフェースとして返却
	return geminiClient, nil
}

// BuildReviewPromptBuilder は、レビューの種類に応じて適切な ReviewPromptBuilder を構築します。
// レビューの種類（リリースノート用か詳細レビュー用か）は ReviewConfig から決定されると仮定します。
func BuildReviewPromptBuilder(cfg config.ReviewConfig) (*prompts.ReviewPromptBuilder, error) {
	var name string
	var template string

	// 適切なテンプレートを選択するロジック
	switch cfg.ReviewMode {
	case "release":
		name = "release_review"
		template = prompts.ReleasePromptTemplate
	case "detail":
		name = "detail_review"
		template = prompts.DetailPromptTemplate
	default:
		// ここではエラーを返さない設計になっているようですが、
		// cmdパッケージのPreRunEで無効なモードは既に検出されるため、現状維持とします。
	}

	builder := prompts.NewReviewPromptBuilder(name, template)
	if err := builder.Err(); err != nil {
		return nil, fmt.Errorf("レビュープロンプトビルダーの初期化エラー: %w", err)
	}

	slog.Debug("ReviewPromptBuilderを構築しました。", slog.String("template_name", name))

	return builder, nil
}
