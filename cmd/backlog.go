package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"git-gemini-reviewer-go/internal/config"
	"git-gemini-reviewer-go/internal/services"
	"github.com/shouni/go-notifier/pkg/notifier"
	"github.com/spf13/cobra"
)

// backlogCmd 固有のフラグ変数のみを定義
var (
	backlogIssueID string // issueID との競合を避けるため変数名を変更
	noPost         bool
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
	// RootCmd は clibase.Execute の内部で生成されるため、サブコマンドの追加は Execute の引数で行うのが理想。
	// ただし、単体ファイルとしてのinit()の実行順序により、ここでRootCmdにAddCommandするのが一般的です。
	// RootCmd.AddCommand(backlogCmd) // 以前の root.go の実行で処理されることを想定しコメントアウト
	// NOTE: 以前の RootCmd 定義は削除されたため、この行は実行されない可能性があります。
	// Execute() にサブコマンドとして渡されることを前提とします。

	// Backlog 固有のフラグのみをここで定義する
	backlogCmd.Flags().StringVar(&backlogIssueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
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
	// NOTE: グローバル変数 'Flags' (AppFlags) からパラメータを抽出する
	params := CreateReviewConfigParams{
		ReviewMode:       Flags.ReviewMode,
		GeminiModel:      Flags.GeminiModel,
		GitCloneURL:      Flags.GitCloneURL,
		BaseBranch:       Flags.BaseBranch,
		FeatureBranch:    Flags.FeatureBranch,
		SSHKeyPath:       Flags.SSHKeyPath,
		LocalPath:        Flags.LocalPath,
		SkipHostKeyCheck: Flags.SkipHostKeyCheck,
	}

	// NOTE: CreateReviewConfig は他の場所で定義されていると仮定
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
		log.Println("✅ Diff がないためレビューをスキップしました。")
		return nil // Diffなしでスキップ
	}

	// 4. no-post フラグによる出力分岐
	if noPost {
		printReviewResult(reviewResult)
		return nil
	}

	// 5. Backlog投稿処理の準備
	if backlogIssueID == "" {
		return fmt.Errorf("--issue-id フラグが指定されていません。Backlogに投稿するには必須です。")
	}

	// 投稿内容の整形
	finalContent := formatBacklogComment(backlogIssueID, cfg, reviewResult)

	// 6. Backlog投稿を実行
	// NOTE: sharedClient を利用するように変更
	err = postToBacklog(ctx, backlogSpaceURL, backlogAPIKey, backlogIssueID, finalContent)
	if err != nil {
		// 投稿に失敗した場合、エラーログを出力し、レビュー結果をコンソールに出力
		log.Printf("ERROR: Backlog へのコメント投稿に失敗しました (課題ID: %s): %v\n", backlogIssueID, err)
		printReviewResult(reviewResult) // ここで呼び出されています
		return fmt.Errorf("Backlog課題 %s へのコメント投稿に失敗しました。詳細は上記レビュー結果を確認してください。", backlogIssueID)
	}

	fmt.Printf("✅ レビュー結果を Backlog 課題 ID: %s に投稿しました。\n", backlogIssueID)
	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// postToBacklog は、Backlogへの投稿処理の責務を持ちます。
// NOTE: sharedClient (*client.Client) を使用するように修正
func postToBacklog(ctx context.Context, url, apiKey, issueID, content string) error {
	// 以前記憶した initAppPreRunE で初期化される sharedClient を利用
	if sharedClient == nil {
		// 万が一初期化されていない場合（テストなど）のフォールバック
		// NOTE: sharedClient の初期化は clibase のライフサイクルに依存するため、実行時に nil の場合、エラーとして処理する方が安全
		// 便宜上、ここではフォールバックの代わりにエラーを返します
		return fmt.Errorf("内部エラー: HTTP クライアント (sharedClient) が初期化されていません")
	}

	backlogService, err := notifier.NewBacklogNotifier(*sharedClient, url, apiKey)
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
