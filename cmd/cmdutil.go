package cmd

import (
	"fmt"
)

// printReviewResult は noPost 時に結果を標準出力します。
func printReviewResult(result string) {
	// 標準出力 (fmt.Println) は維持
	fmt.Println("\n--- Gemini AI レビュー結果 (投稿スキップまたは投稿失敗) ---")
	fmt.Println(result)
	fmt.Println("-----------------------------------------------------")
}
