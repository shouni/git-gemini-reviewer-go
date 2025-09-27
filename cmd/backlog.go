package cmd

import (
	"fmt"
	"os"

	// os, filepath などのインポートは必要に応じて追加
	"github.com/spf13/cobra"
)

// 💡 注意: generic.go で定義した ReviewConfig を共有する仕組みが必要です。
// ここでは簡易的に、Backlog固有のフラグのみを定義します。

var backlogCfg struct {
	ReviewConfig
	NoPost bool // Backlogモード固有のフラグ
}

func init() {
	RootCmd.AddCommand(backlogCmd)

	// generic.go と同じ引数定義をここにも行う必要がありますが、コードが冗長になるため、
	// 実際には共通の setupFlags 関数や別のライブラリ（viperなど）を使って簡略化します。
	// ここでは generic.go のフラグ定義と同じ内容をベースとしてください。

	// 例: 必須フラグの定義
	backlogCmd.Flags().StringVar(&backlogCfg.GitCloneURL, "git-clone-url", "", "レビュー対象のGitリポジトリURL")
	backlogCmd.MarkFlagRequired("git-clone-url")
	// ... 他の必須フラグも定義 ...

	// Backlogモード固有のフラグ
	backlogCmd.Flags().BoolVar(&backlogCfg.NoPost, "no-post", false,
		"レビュー結果をBacklogにコメント投稿するのをスキップします。")

	// Backlog投稿時には通常必須となるが、--no-post時は任意となるフラグ
	backlogCmd.Flags().StringVar(&backlogCfg.IssueID, "issue-id", "",
		"関連する課題ID (Backlog投稿時には必須/スキップ時は任意)")
}

var backlogCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Gitリポジトリの差分をレビューし、Backlogにコメント投稿します。",
	Long:  `このモードは、差分レビューの結果をBacklogの課題にコメントとして投稿します。`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("--- ✅ Backlogレビューモードを実行しました ---")

		// --- Pythonの投稿ロジック再現 ---
		if backlogCfg.NoPost {
			fmt.Println("--- ⚠️ `--no-post`が指定されました。Backlogへのコメント投稿はスキップし、結果は標準出力されます。 ---")
			// GitCodeReviewer(args) の Go版実装を呼び出す
		} else if backlogCfg.IssueID == "" {
			// IssueIDがない場合、投稿できないためエラーとして終了
			fmt.Fprintln(os.Stderr, "エラー: Backlogへコメント投稿するには `--issue-id` が必須です。投稿をスキップする場合は `--no-post` を指定してください。")
			os.Exit(1)
		} else {
			// BacklogCodeReviewer(args) の Go版実装を呼び出す
		}

		fmt.Printf("設定:\n%+v\n", backlogCfg)
	},
}
