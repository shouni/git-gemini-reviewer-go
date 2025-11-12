package pipeline

import (
	"context"
	"fmt"
	"log"
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
) (string, error) { // config.ReviewConfig は設定値として維持

	log.Println("--- 1. Gitリポジトリのセットアップと差分取得を開始 ---")
	fmt.Println("🔍 Gitリポジトリを準備し、差分を取得中...")

	// 2.1. クローン/アップデート
	repo, err := gitService.CloneOrUpdate(cfg.GitCloneURL)
	if err != nil {
		log.Printf("ERROR: Gitリポジトリのセットアップに失敗しました: %v", err)
		return "", fmt.Errorf("Gitリポジトリのクローン/更新に失敗しました: %w", err)
	}

	// defer処理
	defer func() {
		if cleanupErr := gitService.Cleanup(repo); cleanupErr != nil {
			log.Printf("Warning: Failed to cleanup local repository: %v", cleanupErr)
		}
	}()

	// 2.2. フェッチ
	if err := gitService.Fetch(repo); err != nil {
		log.Printf("ERROR: 最新の変更のフェッチに失敗しました: %v", err)
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// 2.3. コード差分を取得
	diffContent, err := gitService.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch)
	if err != nil {
		log.Printf("ERROR: Git差分の取得に失敗しました: %v", err)
		return "", fmt.Errorf("Git差分の取得に失敗しました: %w", err)
	}

	if strings.TrimSpace(diffContent) == "" {
		fmt.Println("ℹ️ 差分が見つかりませんでした。レビューをスキップします。")
		return "", nil
	}

	log.Printf("Git差分の取得に成功しました。サイズ: %dバイト\n", len(diffContent))

	// 3. プロンプトの組み立て
	promptBuilder := prompts.NewReviewPromptBuilder(cfg.PromptContent)

	finalPrompt, err := promptBuilder.Build(diffContent)
	if err != nil {
		log.Printf("ERROR: プロンプトの組み立てエラー: %v", err)
		return "", fmt.Errorf("プロンプトの組み立てに失敗しました: %w", err)
	}

	// --- 4. AIレビュー ---
	fmt.Println("🚀 Gemini AIによるコードレビューを開始します...")

	// 4.2. レビューの依頼
	reviewComment, err := geminiService.ReviewCodeDiff(ctx, finalPrompt)
	if err != nil {
		log.Printf("ERROR: Geminiによるコードレビュー中にエラーが発生しました: %v", err)
		return "", fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}

	log.Println("AIレビューの取得に成功しました。")

	return reviewComment, nil
}
