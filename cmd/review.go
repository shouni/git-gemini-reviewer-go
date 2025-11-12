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
// これにより、CLIコマンドの RunE ロジックをクリーンに保ちます。
func executeReviewPipeline(
	ctx context.Context,
	cfg config.ReviewConfig,
	logger *slog.Logger, // ロガーも DI のように渡すと便利
) error {

	// --- 1. 依存関係の構築（Builder パッケージを使用） ---

	// 1.1. Git Service の構築
	gitService := builder.BuildGitService(cfg, logger)

	// 1.2. Gemini Service の構築
	geminiService, err := builder.BuildGeminiService(ctx, cfg, logger)
	if err != nil {
		// 構築時のエラーをラップして返します
		return fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// --- 2. 共通ロジック (Pipeline) の実行 ---

	// 依存サービスを注入して RunReviewAndGetResult を呼び出す
	reviewResult, err := pipeline.RunReviewAndGetResult(
		ctx,
		cfg,
		gitService,    // 依存性注入 (DI)
		geminiService, // 依存性注入 (DI)
	)
	if err != nil {
		// パイプライン実行時のエラーをそのまま返します
		return err
	}

	// --- 3. 結果の処理 ---
	// ここでレビュー結果 (reviewResult) を使った後続処理を行います。
	if reviewResult != "" {
		fmt.Println("\n--- AI CODE REVIEW RESULT ---")
		fmt.Println(reviewResult)
		// 例: Slack/GitHubへの投稿処理など
	} else {
		// パイプラインがスキップされた場合など
		logger.Info("レビューパイプラインはスキップされました（差分なしなど）。")
	}

	return nil
}

// -------------------------------------------------------------
// ▼ 呼び出し元 (Cobraの RunE など) での利用例
// -------------------------------------------------------------

/*
// 例: reviewCmd の RunE 関数
func reviewRunE(cmd *cobra.Command, args []string) error {
    // ReviewConfig と logger がこのスコープで利用可能であることを前提とする
    cfg := ReviewConfig // 以前のコードの ReviewConfig に該当
    logger := getLogger() // ロガーを取得する関数を仮定

    // 新しく定義した関数を呼び出す
    return executeReviewPipeline(cmd.Context(), cfg, logger)
}
*/
