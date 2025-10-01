package internal

import (
	"context"
	"fmt"
	"log" // log.Printf を使用

	// services パッケージは、一つ上の階層の git-gemini-reviewer-go から見た相対パスでインポート。
	// 実際のプロジェクト構造に合わせて調整が必要です。
	"git-gemini-reviewer-go/internal/services"
)

// ReviewParams はレビューを実行するために必要な設定パラメータを保持します。
// CLIコマンドのフラグから直接値を受け取ることを想定しています。
type ReviewParams struct {
	RepoURL        string // GitリポジトリのURL（Clone/Openに必要）
	LocalPath      string // Gitリポジトリのローカルパス
	SSHKeyPath     string // SSH認証に必要
	BaseBranch     string // 比較基準ブランチ
	FeatureBranch  string // レビュー対象ブランチ
	ModelName      string // Geminiモデル名
	PromptFilePath string // プロンプトファイルのパス
	IssueID        string // Backlogなどで使用（今回のコアロジックでは未使用）
	InsecureSkipHostKeyCheck bool   // SSHホストキー検証をスキップするかどうか
}

// ReviewResult は AI レビューの最終結果を保持します。
type ReviewResult struct {
	ReviewComment string
	DiffSize      int
	ModelName     string
}

// RunReviewer はGitの差分を取得し、Geminiにレビューを依頼するコアロジックを実行します。
// この関数は、GitリポジトリのセットアップからAIレビューまでの一連の処理を調整する役割を担います。
func RunReviewer(ctx context.Context, params ReviewParams) (*ReviewResult, error) {

	// 1. Gitクライアントの初期化とリポジトリのセットアップ
	log.Println("--- 1. Gitリポジトリのセットアップと差分取得を開始 ---")

	gitClient := services.NewGitClient(params.LocalPath, params.SSHKeyPath)
	gitClient.BaseBranch = params.BaseBranch

	// 1.1. 外部コマンドでクローンを実行し、リポジトリインスタンスを取得
	repo, err := gitClient.CloneOrUpdateWithExec(params.RepoURL, params.LocalPath)
	if err != nil {
		// 修正: log.Errorf -> log.Printf に変更し、fmt.Errorf でエラーを返す
		log.Printf("ERROR: Gitリポジトリのセットアップに失敗しました: %v", err)
		return nil, fmt.Errorf("Gitリポジトリのセットアップに失敗しました: %w", err)
	}

	// 1.2. 最新の変更をフェッチ
	if err := gitClient.Fetch(repo); err != nil {
		// 修正: log.Errorf -> log.Printf に変更し、fmt.Errorf でエラーを返す
		log.Printf("ERROR: 最新の変更のフェッチに失敗しました: %v", err)
		return nil, fmt.Errorf("最新の変更のフェッチに失敗しました: %w", err)
	}

	// 1.3. コード差分を取得
	diffContent, err := gitClient.GetCodeDiff(repo, params.BaseBranch, params.FeatureBranch)
	if err != nil {
		// 修正: log.Errorf -> log.Printf に変更し、fmt.Errorf でエラーを返す
		log.Printf("ERROR: Git差分の取得に失敗しました: %v", err)
		return nil, fmt.Errorf("Git差分の取得に失敗しました: %w", err)
	}

	if diffContent == "" {
		log.Println("レビュー対象の差分がありませんでした。処理を終了します。")
		// 差分がない場合はエラーではないため、nilを返して成功終了
		return nil, nil
	}

	log.Println("Git差分の取得に成功しました。")
	log.Printf("取得したDiffのサイズ: %dバイト\n", len(diffContent))

	// --- 2. AIレビュー（Gemini） ---
	log.Println("--- 2. AIレビュー（Gemini）を開始 ---")

	// リファクタリングされた services.NewGeminiClient を使用
	geminiClient, err := services.NewGeminiClient(params.ModelName)
	if err != nil {
		// 修正: log.Errorf -> log.Printf に変更し、fmt.Errorf でエラーを返す
		log.Printf("ERROR: Geminiクライアントの初期化エラー: %v", err)
		return nil, fmt.Errorf("Geminiクライアントの初期化エラー: %w", err)
	}
	defer func() {
		if geminiClient != nil { // 念のためnilチェック
			geminiClient.Close()
		}
	}()

	// 2.1. レビューの依頼
	reviewComment, err := geminiClient.ReviewCodeDiff(ctx, diffContent, params.PromptFilePath)
	if err != nil {
		// ここはすでに fmt.Errorf で適切にエラーを返していたため、log.Printf を追加
		log.Printf("ERROR: Geminiによるコードレビュー中にエラーが発生しました: %v", err)
		return nil, fmt.Errorf("Geminiによるコードレビュー中にエラーが発生しました: %w", err)
	}

	log.Println("AIレビューの取得に成功しました。")

	// --- 3. 結果を返す ---
	log.Println("レビュー処理を完了しました。")

	return &ReviewResult{
		ReviewComment: reviewComment,
		DiffSize:      len(diffContent),
		ModelName:     params.ModelName,
	}, nil
}
