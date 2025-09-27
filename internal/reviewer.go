package internal

import (
	"context"
	"fmt"
	"log"

	"git-gemini-reviewer-go/internal/services"
)

// ReviewParams は CLI から受け取る全てのパラメータを保持する構造体です。
// (エラーメッセージ internal/reviewer.go:13:25: undefined: ReviewParams を解消)
type ReviewParams struct {
	RepoName      string
	BaseBranch    string
	FeatureBranch string
	IssueID       string
	LocalPath     string
	ModelName     string
}

// RunReviewer は実際のレビューロジックを実行する関数です。
func RunReviewer(params ReviewParams) error {

	// --- 1. Git差分の取得 ---
	gitManager := services.NewGitManager(params.LocalPath)
	log.Println("--- 1. Git差分の取得を開始 ---")

	// diffContent はここで定義されます (エラーメッセージ internal/reviewer.go:25:68: undefined: diffContent を解消)
	diffContent, err := gitManager.GetDiff(params.BaseBranch, params.FeatureBranch)
	if err != nil {
		return fmt.Errorf("Git差分の取得に失敗しました: %w", err)
	}
	log.Println("Git差分の取得に成功しました。")
	log.Printf("取得したDiffのサイズ: %dバイト", len(diffContent))

	// --- 2. AIレビュー（Gemini） ---
	log.Println("--- 2. AIレビュー（Gemini）を開始 ---")

	geminiClient, err := services.NewGeminiClient(params.ModelName)
	if err != nil {
		return fmt.Errorf("Geminiクライアントの初期化エラー: %w", err)
	}

	// context.Background()を最初の引数として渡すように修正
	_, err = geminiClient.ReviewCodeDiff(context.Background(), diffContent)
	if err != nil {
		return fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}

	log.Println("AIレビューの取得に成功しました。")

	//// --- 3. Backlogコメント投稿（モック対応済み） ---
	//log.Println("--- 3. Backlogコメント投稿を開始 ---")
	//
	//// 投稿するコメント本文を構築
	//finalComment := fmt.Sprintf("## AIコードレビュー結果\n\n**Geminiモデル (%s) によるレビュー:**\n\n%s",
	//	params.ModelName,
	//	reviewComment, // ← Geminiから受け取ったレビューコメント
	//)
	//
	//// コメント投稿の実行 (モックモードの場合はコンソールに出力)
	//if err := backlogClient.PostComment(params.IssueID, finalComment); err != nil {
	//	return fmt.Errorf("Backlogへのコメント投稿に失敗しました: %w", err)
	//}
	//
	//log.Printf("Backlog課題 %s へのコメント投稿処理を完了しました。", params.IssueID)

	log.Println("レビュー処理を完了しました。")
	return nil
}
