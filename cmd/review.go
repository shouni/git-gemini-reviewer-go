package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"git-gemini-reviewer-go/internal/builder"
	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/pipeline"
)

// cleanURLRegex はファイルシステムで使用できない文字を特定するための正規表現です。
var cleanURLRegex = regexp.MustCompile(`[^\w\-.]+`)

// executeReviewPipeline は、すべての依存関係を構築し、レビューパイプラインを実行します。
// 実行結果の文字列とエラーを返します。
func executeReviewPipeline(
	ctx context.Context,
	cfg config.ReviewConfig,
) (string, error) {

	// --- 1. 依存関係の構築（Builder パッケージを使用） ---
	if ReviewConfig.LocalPath == "" {
		ReviewConfig.LocalPath = GenerateLocalPathFromURL(cfg.RepoURL)
		slog.Debug("LocalPathが未指定のため、URLから動的にパスを生成しました。", "generated_path", ReviewConfig.LocalPath)
	}

	// --- 2. 依存関係の構築（Builder パッケージを使用） ---
	gitService := builder.BuildGitService(cfg)

	geminiService, err := builder.BuildGeminiService(ctx, cfg)
	if err != nil {
		return "", fmt.Errorf("Gemini Service の構築に失敗しました: %w", err)
	}

	// --- 3. promptBuilder の構築
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

// GenerateLocalPathFromURL は、リポジトリURLから一意で安全なローカルパスを生成します。
// これは、ユーザーが --local-path を指定しなかった場合のデフォルト値を設定するために使用されます。
func GenerateLocalPathFromURL(repoURL string) string {
	// ベースディレクトリを設定 (例: /tmp/git-reviewer-repos)
	tempBase := os.TempDir() + "/git-reviewer-repos"

	// 1. スキームと.gitを削除してクリーンな名前を取得
	name := strings.TrimSuffix(repoURL, ".git")
	name = strings.TrimPrefix(name, "https://")
	name = strings.TrimPrefix(name, "http://")
	name = strings.TrimPrefix(name, "git@")

	// 2. パスとして使用できない文字をハイフンに置換
	name = cleanURLRegex.ReplaceAllString(name, "-")

	// 3. 衝突防止のため、URL全体のSHA-256ハッシュの先頭8桁を追加
	hasher := sha256.New()
	hasher.Write([]byte(repoURL))
	hash := fmt.Sprintf("%x", hasher.Sum(nil))[:8]

	// パス名が長くなりすぎないように調整し、ハイフンをトリム
	safeDirName := fmt.Sprintf("%s-%s", strings.Trim(name, "-"), hash)

	// 4. ベースパスと結合して返す
	return filepath.Join(tempBase, safeDirName)
}
