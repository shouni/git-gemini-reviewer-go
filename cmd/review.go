package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"git-gemini-reviewer-go/internal/builder"
	"git-gemini-reviewer-go/internal/config"

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

	// --- 2. 実行ロジック (Runner層) の構築 ---
	reviewRunner, err := builder.BuildReviewRunner(ctx, cfg)
	if err != nil {
		// BuildReviewRunner が内部でアダプタやビルダーの構築エラーをラップして返す
		return "", fmt.Errorf("レビュー実行器の構築に失敗しました: %w", err)
	}

	slog.Info("レビューパイプラインを開始します。")

	// --- 3. 実行ロジック (Runner層) の実行 ---
	reviewResult, err := reviewRunner.Run(ctx, cfg)
	if err != nil {
		return "", err
	}

	// --- 4. 結果の返却 ---
	if reviewResult == "" {
		slog.Info("Diff がないためレビューをスキップしました。")
		return "", nil
	}

	return reviewResult, nil
}
