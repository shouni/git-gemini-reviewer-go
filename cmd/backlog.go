package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"time" // httpclient.New() のために time パッケージをインポート

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/services"
	"github.com/shouni/go-notifier/pkg/notifier"
	"github.com/shouni/go-web-exact/pkg/httpclient" // httpclientをインポート

	"github.com/spf13/cobra"
)

// backlogCmd 固有のフラグ変数のみを定義
var (
	issueID string
	noPost  bool
)

// backlogCmd は、レビュー結果を Backlog にコメントとして投稿するコマンドです。
var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "コードレビューを実行し、その結果をBacklogにコメントとして投稿します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をBacklogの指定された課題にコメントとして自動で投稿します。`,
	// ロジックを外部関数に分離
	RunE: runBacklogCommand,
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// Backlog 固有のフラグのみをここで定義する
	backlogCmd.Flags().StringVar(&issueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
	backlogCmd.Flags().BoolVar(&noPost, "no-post", false, "投稿をスキップし、結果を標準出力する")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runBacklogCommand はコマンドの主要な実行ロジックを含みます。
func runBacklogCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. 環境変数の確認
	backlogAPIKey := os.Getenv("BACKLOG_API_KEY")
	backlogSpaceURL := os.Getenv("BACKLOG_SPACE_URL")

	if backlogAPIKey == "" || backlogSpaceURL == "" {
		return fmt.Errorf("Backlog連携には環境変数 BACKLOG_API_KEY および BACKLOG_SPACE_URL が必須です")
	}

	// 2. 共通設定の作成
	// NOTE: グローバル変数からパラメータを抽出する
	params := CreateReviewConfigParams{
		ReviewMode:       reviewMode,
		GeminiModel:      geminiModel,
		GitCloneURL:      gitCloneURL,
		BaseBranch:       baseBranch,
		FeatureBranch:    featureBranch,
		SSHKeyPath:       sshKeyPath,
		LocalPath:        localPath,
		SkipHostKeyCheck: skipHostKeyCheck,
	}

	cfg, err := CreateReviewConfig(params)
	if err != nil {
		return err // 無効なレビューモードのエラーを処理
	}

	// 3. 共通ロジックを実行し、結果を取得
	reviewResult, err := services.RunReviewAndGetResult(ctx, cfg)
	if err != nil {
		return err
	}

	if reviewResult == "" {
		return nil // Diffなしでスキップ
	}

	// 4. no-post フラグによる出力分岐
	if noPost {
		printReviewResult(reviewResult)
		return nil
	}

	// 5. Backlog投稿処理の準備
	if issueID == "" {
		return fmt.Errorf("--issue-id フラグが指定されていません。Backlogに投稿するには必須です。")
	}

	// 投稿内容の整形
	finalContent := formatBacklogComment(issueID, cfg, reviewResult)

	// 6. Backlog投稿を実行
	err = postToBacklog(ctx, backlogSpaceURL, backlogAPIKey, issueID, finalContent)
	if err != nil {
		// 投稿に失敗した場合、エラーログを出力し、レビュー結果をコンソールに出力
		log.Printf("ERROR: Backlog へのコメント投稿に失敗しました (課題ID: %s): %v\n", issueID, err)
		printReviewResult(reviewResult) // ここで呼び出されています
		return fmt.Errorf("Backlog課題 %s へのコメント投稿に失敗しました。詳細は上記レビュー結果を確認してください。", issueID)
	}

	fmt.Printf("✅ レビュー結果を Backlog 課題 ID: %s に投稿しました。\n", issueID)
	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// postToBacklog は、Backlogへの投稿処理の責務を持ちます。
func postToBacklog(ctx context.Context, url, apiKey, issueID, content string) error {
	// httpclient.New() と time.Second を使用
	httpClient := httpclient.New(30 * time.Second)

	backlogService, err := notifier.NewBacklogNotifier(httpClient, url, apiKey)
	if err != nil {
		return fmt.Errorf("Backlogクライアントの初期化に失敗しました: %w", err)
	}

	fmt.Printf("📤 Backlog 課題 ID: %s にレビュー結果を投稿します...\n", issueID)

	// PostComment はリトライロジックを持つ
	return backlogService.PostComment(ctx, issueID, content)
}

// formatBacklogComment はコメントのヘッダーと本文を整形します。
// cfg の型は config.ReviewConfig に依存
func formatBacklogComment(issueID string, cfg config.ReviewConfig, reviewResult string) string {
	// 課題番号、リポジトリ名、ブランチ情報を整形
	// NOTE: cfg はポインタではなく値として渡されていると仮定し、* を外しました
	header := fmt.Sprintf(
		"### AI コードレビュー結果\n\n"+
			"**対象課題ID:** `%s`\n"+
			"**基準ブランチ:** `%s`\n"+
			"**レビュー対象ブランチ:** `%s`\n\n"+
			"---\n",
		issueID,
		cfg.BaseBranch,
		cfg.FeatureBranch,
	)

	// ヘッダーとレビュー結果を結合
	return header + reviewResult
}

// printReviewResult は noPost 時に結果を標準出力します。
func printReviewResult(result string) {
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
