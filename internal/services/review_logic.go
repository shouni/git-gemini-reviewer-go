package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/prompts"
)

// RunReviewAndGetResult はGit Diffを取得し、Gemini AIでレビューを実行します。
// 投稿は行わず、レビュー結果の文字列のみを返します。
func RunReviewAndGetResult(ctx context.Context, cfg config.ReviewConfig) (string, error) {

	log.Println("--- 1. Gitリポジトリのセットアップと差分取得を開始 ---")
	fmt.Println("🔍 Gitリポジトリを準備し、差分を取得中...")

	// 2. Gitクライアントの初期化とセットアップを分離したヘルパー関数で実行
	gitClient := setupGitClient(cfg)

	// 2.1. クローン/アップデート
	repo, err := gitClient.CloneOrUpdate(cfg.GitCloneURL)
	if err != nil {
		log.Printf("ERROR: Gitリポジトリのセットアップに失敗しました: %v", err)
		return "", fmt.Errorf("Gitリポジトリのクローン/更新に失敗しました: %w", err)
	}

	// 2.2. フェッチ
	if err := gitClient.Fetch(repo); err != nil {
		log.Printf("ERROR: 最新の変更のフェッチに失敗しました: %v", err)
		return "", fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// 2.3. コード差分を取得
	diffContent, err := gitClient.GetCodeDiff(repo, cfg.BaseBranch, cfg.FeatureBranch)
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
	// NewReviewPromptBuilder は cfg.PromptContent (テンプレート) を使用
	promptBuilder := prompts.NewReviewPromptBuilder(cfg.PromptContent)

	// diffContent をテンプレートに埋め込み、最終的なプロンプトを生成
	finalPrompt, err := promptBuilder.Build(diffContent)
	if err != nil {
		log.Printf("ERROR: プロンプトの組み立てエラー: %v", err)
		return "", fmt.Errorf("プロンプトの組み立てに失敗しました: %w", err)
	}

	// --- 4. AIレビュー（Gemini: リトライ内蔵） ---
	fmt.Println("🚀 Gemini AIによるコードレビューを開始します...")

	// 4.1. Geminiクライアントの初期化
	geminiClient, err := NewGeminiClient(ctx, cfg.GeminiModel)
	if err != nil {
		log.Printf("ERROR: Geminiクライアントの初期化エラー: %v", err)
		return "", fmt.Errorf("Geminiクライアントの初期化エラー: %w", err)
	}

	// 4.2. レビューの依頼
	reviewComment, err := geminiClient.ReviewCodeDiff(ctx, finalPrompt)
	if err != nil {
		log.Printf("ERROR: Geminiによるコードレビュー中にエラーが発生しました: %v", err)
		return "", fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}

	log.Println("AIレビューの取得に成功しました。")

	return reviewComment, nil
}

// setupGitClient はGitクライアントを初期化し、設定を適用します。
// GitService インターフェースを返します。
func setupGitClient(cfg config.ReviewConfig) GitService {
	// NewGitClient の引数をオプションパターンに合わせる
	opts := []GitClientOption{
		WithInsecureSkipHostKeyCheck(cfg.SkipHostKeyCheck),
	}

	if cfg.BaseBranch != "" {
		opts = append(opts, WithBaseBranch(cfg.BaseBranch))
	}

	gitClient := NewGitClient(
		cfg.LocalPath,
		cfg.SSHKeyPath,
		opts...,
	)

	if cfg.SkipHostKeyCheck {
		// セキュリティに関するログ出力はここに集約
		log.Println("!!! SECURITY ALERT !!! SSH host key checking has been explicitly disabled. This makes connections vulnerable to Man-in-the-Middle attacks. Ensure this is intentional and NOT used in production.")
	}

	return gitClient
}
