package runner

import (
	"context"
	"fmt"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/pkg/adapters"
	"git-gemini-reviewer-go/pkg/prompts"
	"log/slog"
	"strings"
)

// ReviewRunner はコードレビューのビジネスロジックを実行します。
// 必要な依存関係（アダプタ）をフィールドとして保持します。
type ReviewRunner struct {
	gitService    adapters.GitService
	geminiService adapters.CodeReviewAI
	promptBuilder prompts.ReviewPromptBuilder
}

// NewReviewRunner は ReviewRunner の新しいインスタンスを生成します。
// 依存関係はコンストラクタ経由で注入されます。
func NewReviewRunner(
	git adapters.GitService,
	gemini adapters.CodeReviewAI,
	pb prompts.ReviewPromptBuilder,
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
	err := r.gitService.CloneOrUpdate(ctx, cfg.RepoURL)
	if err != nil {
		return "", fmt.Errorf("リポジトリのセットアップに失敗しました: %w", err)
	}

	// クリーンアップを遅延実行 (常に実行を保証)
	defer func() {
		if cleanupErr := r.gitService.Cleanup(ctx); cleanupErr != nil {
			slog.Error("Gitリポジトリのクリーンアップに失敗しました。", "error", cleanupErr)
		}
	}()

	// リモートから最新の変更をフェッチ
	if err := r.gitService.Fetch(ctx); err != nil {
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// コード差分を取得
	codeDiff, err := r.gitService.GetCodeDiff(ctx, cfg.BaseBranch, cfg.FeatureBranch)
	if err != nil {
		return "", fmt.Errorf("コード差分の取得に失敗しました: %w", err)
	}

	if strings.TrimSpace(codeDiff) == "" {
		return "", nil
	}
	slog.Info("Git差分の取得に成功しました。", "size_bytes", len(codeDiff))

	// 5. プロンプトの生成
	slog.InfoContext(ctx, "3. AIプロンプトを生成中...", "mode", cfg.ReviewMode)
	templateData := prompts.TemplateData{DiffContent: codeDiff}
	finalPrompt, err := r.promptBuilder.Build(cfg.ReviewMode, templateData)
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

// 差分がない場合に返す、最小限の静的Markdownメッセージ
func generateNoDiffMessage(base, feature string) string {
	// リリース可否判定や詳細な指摘事項は省略し、総評のみに集中する。
	return fmt.Sprintf("### 1. レビュー結果の概要\n\n"+
		"**【ステータス】** 正常終了 (No Diff)\n\n"+
		"### 2. 総評 (Summary)\n\n"+
		"ベースブランチ ('%s') とフィーチャーブランチ ('%s') 間に有効な差分が見つからなかったため、コードの品質に関する評価は実施できませんでした。AIレビューはスキップされました。\n",
		base,
		feature,
	)
}
