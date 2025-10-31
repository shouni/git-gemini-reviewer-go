package cmd

import (
	"fmt"
	"git-gemini-reviewer-go/internal/services"
	"log"

	"github.com/spf13/cobra"
)

// genericCmd は、リモートリポジトリのブランチ比較を Gemini AI に依頼し、結果を標準出力に出力するコマンドです。
var genericCmd = &cobra.Command{
	Use:   "generic",
	Short: "コードレビューを実行し、その結果を標準出力に出力します。",
	Long:  `このコマンドは、指定されたGitリポジトリのブランチ間の差分をAIでレビューし、その結果を標準出力に直接表示します。Backlogなどの外部サービスとの連携は行いません。`,
	RunE: func(cmd *cobra.Command, args []string) error {

		// 1. パラメータ構造体を作成 (AppFlags (Flags) を利用)
		// NOTE: 以前のソースコードで定義されていたグローバル変数（reviewMode, geminiModel など）
		// の代わりに、AppFlags 構造体 (Flags) のフィールドを利用します。
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

		// 2. CreateReviewConfig を呼び出し、設定オブジェクトを取得
		// NOTE: CreateReviewConfig は他の場所で定義されていると仮定
		cfg, err := CreateReviewConfig(params)
		if err != nil {
			return err // 無効なレビューモードのエラーを処理
		}

		// 3. 共通ロジックを実行し、結果を取得
		reviewResult, err := services.RunReviewAndGetResult(cmd.Context(), cfg)
		if err != nil {
			return err
		}

		if reviewResult == "" {
			log.Println("✅ Diff がないためレビューをスキップしました。")
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
	// genericCmd は root.go の Execute() 関数で clibase.Execute にサブコマンドとして渡されます。
}
