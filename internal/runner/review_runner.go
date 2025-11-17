package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"git-gemini-reviewer-go/internal/adapters"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"
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
	cfg config.ReviewConfig, // 実行時設定を受け取る
) (string, error) {

	slog.Info("Gitリポジトリのセットアップと差分取得を開始します。")
	// 2.1. クローン/アップデート
	repo, err := r.gitService.CloneOrUpdate(cfg.RepoURL) // r.gitService を使用
	if err != nil {
		slog.Error("Gitリポジトリのセットアップに失敗しました。", "error", err, "url", cfg.RepoURL)
		return "", fmt.Errorf("Gitリポジトリのクローン/更新に失敗しました: %w", err)
	}

	defer func() {
		if repo != nil { // repoがnilでないことを確認
			if cleanupErr := r.gitService.Cleanup(repo); cleanupErr != nil { // r.gitService を使用
				slog.Warn("ローカルリポジトリのクリーンアップに失敗しました。", "error", cleanupErr)
			}
		}
	}()

	// 2.2. フェッチ
	if err := r.gitService.Fetch(repo); err != nil { // r.gitService を使用
		slog.Error("最新の変更のフェッチに失敗しました。", "error", err)
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// 2.3. コード差分を取得
	diffContent, err := r.gitService.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch) // r.gitService を使用
	if err != nil {
		slog.Error("Git差分の取得に失敗しました。", "error", err)
		return "", fmt.Errorf("Git差分の取得に失敗しました: %w", err)
	}

	if strings.TrimSpace(diffContent) == "" {
		return "", nil // 差分がない場合は空文字列を返して終了
	}
	slog.Info("Git差分の取得に成功しました。", "size_bytes", len(diffContent))

	// 3. プロンプトの組み立て
	// 注入されたビルダーと新しいデータ構造を使用
	reviewData := prompts.ReviewTemplateData{
		DiffContent: diffContent,
	}

	finalPrompt, err := r.promptBuilder.Build(reviewData) // r.promptBuilder を使用
	if err != nil {
		slog.Error("プロンプトの組み立てエラー。", "error", err)
		return "", fmt.Errorf("プロンプトの組み立てに失敗しました: %w", err)
	}

	// --- 4. AIレビュー ---
	slog.Info("Gemini AIによるコードレビューを開始します。", "model", cfg.GeminiModel)

	// adapters.CodeReviewAI の ReviewCodeDiff メソッドを呼び出す
	reviewComment, err := r.geminiService.ReviewCodeDiff(ctx, finalPrompt)
	if err != nil {
		slog.Error("Geminiによるコードレビュー中にエラーが発生しました。", "error", err)
		return "", fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}
	slog.Info("AIレビューの取得に成功しました。")

	return reviewComment, nil
}
