package cmd

import (
	"log/slog"

	"github.com/spf13/cobra"
)

// genericCmd は、リモートリポジトリのブランチ比較を Gemini AI に依頼し、結果を標準出力に出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果を標準出力に直接表示します。Backlogなどの外部サービスとの連携は行いません。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. パイプラインを実行し、結果を受け取る
		reviewResult, err := executeReviewPipeline(cmd.Context(), ReviewConfig)
		if err != nil {
			return err
		}

		// 2. レビュー結果の出力 (generic 固有の処理)
		// ユーザーの提案に基づき、レビュー結果の内容が空でない場合にのみ標準出力に出力する
		if reviewResult != "" {
			printReviewResult(reviewResult)
			slog.Info("レビュー結果を標準出力に出力しました。")
		} else {
			slog.Info("レビュー結果の内容が空のため、標準出力への出力はスキップしました。")
		}

		return nil
	},
}

func init() {
}
