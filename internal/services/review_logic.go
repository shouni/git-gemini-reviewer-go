package services

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// ReviewConfig はレビュー実行に必要な全てのパラメータを保持します。（以前定義したものを使用）
type ReviewConfig struct {
	GeminiModel      string
	PromptContent    string
	GitCloneURL      string
	BaseBranch       string
	FeatureBranch    string
	SSHKeyPath       string
	LocalPath        string
	SkipHostKeyCheck bool
}

// RunReviewAndGetResult はGit Diffを取得し、Gemini AIでレビューを実行します。
// 投稿は行わず、レビュー結果の文字列のみを返します。
// 以前の RunReviewer 関数と RunReviewAndGetResult 関数を統合したものです。
func RunReviewAndGetResult(ctx context.Context, cfg ReviewConfig) (string, error) {

	log.Println("--- 1. Gitリポジトリのセットアップと差分取得を開始 ---")
	fmt.Println("🔍 Gitリポジトリを準備し、差分を取得中...")

	// 2. Gitクライアントの初期化とセットアップ
	gitClient := NewGitClient(cfg.LocalPath, cfg.SSHKeyPath)
	if cfg.SkipHostKeyCheck {
		log.Println("!!! SECURITY ALERT !!! SSH host key checking has been explicitly disabled. This makes connections vulnerable to Man-in-the-Middle attacks. Ensure this is intentional and NOT used in production.")
		gitClient.InsecureSkipHostKeyCheck = true
	}
	gitClient.BaseBranch = cfg.BaseBranch
	gitClient.InsecureSkipHostKeyCheck = cfg.SkipHostKeyCheck

	// 2.1. クローン/アップデート
	repo, err := gitClient.CloneOrUpdateWithExec(cfg.GitCloneURL, cfg.LocalPath)
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

	// --- 3. AIレビュー（Gemini） ---
	fmt.Println("🚀 Gemini AIによるコードレビューを開始します...")
	geminiClient, err := NewGeminiClient(ctx, cfg.GeminiModel)
	if err != nil {
		log.Printf("ERROR: Geminiクライアントの初期化エラー: %v", err)
		return "", fmt.Errorf("Geminiクライアントの初期化エラー: %w", err)
	}

	// 3.1. レビューの依頼
	reviewComment, err := geminiClient.ReviewCodeDiff(ctx, diffContent, cfg.PromptContent)
	if err != nil {
		log.Printf("ERROR: Geminiによるコードレビュー中にエラーが発生しました: %v", err)
		return "", fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}

	log.Println("AIレビューの取得に成功しました。")

	return reviewComment, nil
}
