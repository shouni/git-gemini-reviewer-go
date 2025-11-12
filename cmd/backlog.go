package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"git-gemini-reviewer-go/internal/config"

	"github.com/shouni/go-notifier/pkg/factory"
	"github.com/spf13/cobra"
)

// --- 構造体: Backlog認証情報 ---

// backlogAuthInfo は、Backlog投稿に必要な認証情報と投稿情報をカプセル化します。
type backlogAuthInfo struct {
	APIKey   string
	SpaceURL string
}

// --- コマンド固有のフラグ変数 ---
var (
	backlogIssueID string // Backlog課題ID。他の issueID との競合を避けるため backlogIssueID としています。
	noPost         bool
)

// backlogCmd は、レビュー結果を Backlog にコメントとして投稿するコマンドです。
var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "コードレビューを実行し、その結果をBacklogにコメントとして投稿します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果をBacklogの指定された課題にコメントとして自動で投稿します。`,
	RunE:  runBacklogCommand,
}

func init() {
	backlogCmd.Flags().StringVar(&backlogIssueID, "issue-id", "", "コメントを投稿するBacklog課題ID（例: PROJECT-123）")
	backlogCmd.Flags().BoolVar(&noPost, "no-post", false, "投稿をスキップし、結果を標準出力する")
}

// --------------------------------------------------------------------------
// コマンドの実行ロジック
// --------------------------------------------------------------------------

// runBacklogCommand はコマンドの主要な実行ロジックを含みます。
func runBacklogCommand(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// 1. 環境変数の確認と構造体へのカプセル化
	authInfo := getBacklogAuthInfo()

	if authInfo.APIKey == "" || authInfo.SpaceURL == "" {
		return fmt.Errorf("Backlog連携には環境変数 BACKLOG_API_KEY および BACKLOG_SPACE_URL が必須です")
	}

	// 2. パイプラインを実行し、結果を受け取る
	reviewResult, err := executeReviewPipeline(cmd.Context(), ReviewConfig, slog.Default())
	if err != nil {
		return err
	}

	// 3. no-post フラグによる出力分岐
	if noPost {
		printReviewResult(reviewResult)
		return nil
	}

	// 4. Backlog投稿の必須フラグ確認
	if backlogIssueID == "" {
		return fmt.Errorf("Backlogに投稿するには --issue-id フラグが必須です")
	}

	// 5. 投稿内容の整形
	finalContent := formatBacklogComment(backlogIssueID, ReviewConfig, reviewResult)

	// 6. Backlog投稿を実行
	err = postToBacklog(ctx, backlogIssueID, finalContent)
	if err != nil {
		slog.Error("Backlogへのコメント投稿に失敗しました。",
			"issue_id", backlogIssueID,
			"error", err,
			"mode", ReviewConfig.ReviewMode)
		printReviewResult(reviewResult)

		return fmt.Errorf("Backlog課題 %s へのコメント投稿処理が失敗しました。詳細はログを確認してください。", backlogIssueID)
	}

	slog.Info("レビュー結果を Backlog 課題にコメント投稿しました。", "issue_id", backlogIssueID)
	return nil
}

// --------------------------------------------------------------------------
// ヘルパー関数
// --------------------------------------------------------------------------

// getBacklogAuthInfo は、環境変数から Backlog 認証情報を取得します。
func getBacklogAuthInfo() backlogAuthInfo {
	return backlogAuthInfo{
		APIKey:   os.Getenv("BACKLOG_API_KEY"),
		SpaceURL: os.Getenv("BACKLOG_SPACE_URL"),
	}
}

// postToBacklog は、Backlogへの投稿処理の責務を持ちます。
func postToBacklog(ctx context.Context, issueID, content string) error {
	// 1. Contextから httpkit.Client を取得 (cmd/root.go の関数を使用)
	httpClient, err := GetHTTPClient(ctx)
	if err != nil {
		slog.Error("🚨 HTTP Clientの取得に失敗しました", "error", err)
		return fmt.Errorf("HTTP Clientの取得に失敗しました: %w", err) // エラーを返す
	}

	// httpClient を使用して依存性を注入
	backlogClient, err := factory.GetBacklogClient(httpClient)
	if err != nil {
		slog.Error("🚨 Backlogクライアントの初期化に失敗しました", "error", err)
		return fmt.Errorf("Backlogクライアントの初期化に失敗しました: %w", err) // エラーを返す
	}
	slog.Info("Backlog課題にレビュー結果を投稿します...", "issue_id", issueID)

	// PostComment はリトライロジックを持つ
	return backlogClient.PostComment(ctx, issueID, content)
}

// formatBacklogComment はコメントのヘッダーと本文を整形します。
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
