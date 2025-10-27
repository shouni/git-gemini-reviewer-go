package cmd

import (
	"fmt"

	"git-gemini-reviewer-go/internal/services"

	"github.com/spf13/cobra"
)

// genericCmd は、リモートリポジトリのブランチ比較を Gemini AI に依頼し、結果を標準出力に出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果を標準出力に直接表示します。Backlogなどの外部サービスとの連携は行いません。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. パラメータ構造体を作成 (グローバル変数への依存をここで解決)
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

		// 2. 修正された CreateReviewConfig を呼び出し、引数を渡す
		cfg, err := CreateReviewConfig(params)
		if err != nil {
			return err // 無効なレビューモードのエラーを処理
		}

		// 3. 共通ロジックを実行し、結果を取得
		// NOTE: services.RunReviewAndGetResult のシグネチャが config.ReviewConfig を受け取るように
		// services パッケージ側で修正されている必要があります。
		reviewResult, err := services.RunReviewAndGetResult(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		if reviewResult == "" {
			return nil // Diffなしでスキップされた場合
		}

		// 4. レビュー結果の出力 (generic 固有の処理)
		fmt.Println("\n--- Gemini AI レビュー結果 ---")
		fmt.Println(reviewResult)
		fmt.Println("------------------------------")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(genericCmd)
}
