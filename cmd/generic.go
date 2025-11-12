package cmd

import (
	"fmt"
	"git-gemini-reviewer-go/internal/pipeline"
	"log/slog"

	"git-gemini-reviewer-go/internal/services"

	"github.com/spf13/cobra"
)

// genericCmd は、リモートリポジトリのブランチ比較を Gemini AI に依頼し、結果を標準出力に出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果を標準出力に直接表示します。Backlogなどの外部サービスとの連携は行いません。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. 共通ロジックを実行し、結果を取得
		// ReviewConfig は initAppPreRunE で既に構築・反映済み
		reviewResult, err := pipeline.RunReviewAndGetResult(cmd.Context(), ReviewConfig)
		if err != nil {
			return err
		}

		if reviewResult == "" {
			slog.Info("Diff がないためレビューをスキップしました。")
			return nil
		}

		// 3. レビュー結果の出力 (generic 固有の処理)
		// NOTE: このセクションは標準出力に結果を出すというコア機能のため、fmt.Println を維持
		fmt.Println("\n--- Gemini AI レビュー結果 ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------")

		// 成功ログを slog で出力
		slog.Info("レビュー結果を標準出力に出力しました。")

		return nil
	},
}

func init() {
}
