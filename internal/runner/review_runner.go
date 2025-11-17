package runner

import (
	"context"
	"fmt"
	"git-gemini-reviewer-go/internal/adapters"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"
	"log/slog"
)

// ReviewRunner はコードレビューのビジネスロジックを実行します。
// 必要な依存関係（アダプタ）をフィールドとして保持します。
type ReviewRunner struct {
	gitService    adapters.GitService
	geminiService adapters.CodeReviewAI
	promptBuilder *prompts.ReviewPromptBuilder
}

// NewReviewRunner は ReviewRunner の新しいインスタンスを生成します。
// 依存関係はコンストラクタ経由で注入されます。
func NewReviewRunner(
	git adapters.GitService,
	gemini adapters.CodeReviewAI,
	pb *prompts.ReviewPromptBuilder,
) *ReviewRunner {
	return &ReviewRunner{
		gitService:    git,
		geminiService: gemini,
		promptBuilder: pb,
	}
}

// Run はGit Diffを取得し、Gemini AIでレビューを実行します。
// 以前の RunReviewAndGetResult のロジックを引き継ぎます。
func (r *ReviewRunner) Run(
	ctx context.Context,
	cfg config.ReviewConfig,
) (string, error) {

	slog.Info("Gitリポジトリのセットアップと差分取得を開始します。")
	// Gitリポジトリのクローンまたは更新
	repo, err := r.gitService.CloneOrUpdate(cfg.RepoURL)
	if err != nil {
		return "", fmt.Errorf("リポジトリのセットアップに失敗しました: %w", err)
	}

	// クリーンアップを遅延実行 (常に実行を保証)
	defer func() {
		if cleanupErr := r.gitService.Cleanup(repo); cleanupErr != nil {
			slog.Error("Gitリポジトリのクリーンアップに失敗しました。", "error", cleanupErr)
		}
	}()

	// リモートから最新の変更をフェッチ
	if err := r.gitService.Fetch(repo); err != nil {
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// コード差分を取得
	diffContent, err := r.gitService.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch)
	if err != nil {
		return "", fmt.Errorf("コード差分の取得に失敗しました: %w", err)
	}

	if diffContent == "" {
		return "", fmt.Errorf("ベースブランチ '%s' とフィーチャーブランチ '%s' 間に差分が見つかりませんでした。", cfg.BaseBranch, cfg.FeatureBranch)
	}

	// プロンプトの組み立て
	// 注入されたビルダーと新しいデータ構造を使用
	reviewData := prompts.ReviewTemplateData{
		DiffContent: diffContent,
	}

	finalPrompt, err := r.promptBuilder.Build(reviewData)
	if err != nil {
		return "", fmt.Errorf("プロンプトの組み立てに失敗しました: %w", err)
	}

	// AIレビューの実行
	slog.Info("Gemini AIによるコードレビューを開始します。", "model", cfg.GeminiModel)

	// Gemini Adapterにレビューを依頼
	reviewResult, err := r.geminiService.ReviewCodeDiff(ctx, finalPrompt)
	if err != nil {
		return "", fmt.Errorf("AIレビューの実行に失敗しました: %w", err)
	}

	return reviewResult, nil
}
