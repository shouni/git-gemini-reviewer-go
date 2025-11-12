package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"

	"git-gemini-reviewer-go/internal/geminiclient"
	"git-gemini-reviewer-go/internal/gitclient"
)

// RunReviewAndGetResult はGit Diffを取得し、Gemini AIでレビューを実行します。
func RunReviewAndGetResult(
	ctx context.Context,
	cfg config.ReviewConfig,
	gitService gitclient.Service,
	geminiService geminiclient.Service,
) (string, error) {

	slog.Info("Gitリポジトリのセットアップと差分取得を開始します。")
	// 2.1. クローン/アップデート
	repo, err := gitService.CloneOrUpdate(cfg.GitCloneURL)
	if err != nil {
		slog.Error("Gitリポジトリのセットアップに失敗しました。", "error", err, "url", cfg.GitCloneURL)
		return "", fmt.Errorf("Gitリポジトリのクローン/更新に失敗しました: %w", err)
	}

	defer func() {
		if repo != nil { // repoがnilでないことを確認
			if cleanupErr := gitService.Cleanup(repo); cleanupErr != nil {
				slog.Warn("ローカルリポジトリのクリーンアップに失敗しました。", "error", cleanupErr)
			}
		}
	}()

	// 2.2. フェッチ
	if err := gitService.Fetch(repo); err != nil {
		slog.Error("最新の変更のフェッチに失敗しました。", "error", err)
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// 2.3. コード差分を取得
	diffContent, err := gitService.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch)
	if err != nil {
		slog.Error("Git差分の取得に失敗しました。", "error", err)
		return "", fmt.Errorf("Git差分の取得に失敗しました: %w", err)
	}

	if strings.TrimSpace(diffContent) == "" {
		return "", nil
	}
	slog.Info("Git差分の取得に成功しました。", "size_bytes", len(diffContent))

	// 3. プロンプトの組み立て
	promptBuilder := prompts.NewReviewPromptBuilder(cfg.PromptContent)
	finalPrompt, err := promptBuilder.Build(diffContent)
	if err != nil {
		slog.Error("プロンプトの組み立てエラー。", "error", err)
		return "", fmt.Errorf("プロンプトの組み立てに失敗しました: %w", err)
	}

	// --- 4. AIレビュー ---
	slog.Info("Gemini AIによるコードレビューを開始します。", "model", cfg.GeminiModel)
	reviewComment, err := geminiService.ReviewCodeDiff(ctx, finalPrompt)
	if err != nil {
		slog.Error("Geminiによるコードレビュー中にエラーが発生しました。", "error", err)
		return "", fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}
	slog.Info("AIレビューの取得に成功しました。")

	return reviewComment, nil
}
