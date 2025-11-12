package cmd

import (
	"fmt"
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
		reviewResult, err := executeReviewPipeline(cmd.Context(), ReviewConfig, slog.Default())
		if err != nil {
			return err
		}

		// 2. レビュー結果の出力 (generic 固有の処理)
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
