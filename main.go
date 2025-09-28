package main

import (
	"flag"
	"fmt"
	"git-gemini-reviewer-go/cmd" // 🚀 CLIのエントリポイント
	"os"
	"path/filepath"
)

// ReviewConfig はコマンドライン引数を保持する構造体です。
// cmd パッケージ内のサブコマンドでフラグをバインドするために使用されます。
type ReviewConfig struct {
	// 必須引数
	GitCloneURL   string
	BaseBranch    string
	FeatureBranch string

	// 任意の引数 (デフォルト値あり)
	LocalPath       string
	IssueID         string
	GeminiModelName string

	// Backlog連携のフラグ
	NoPost bool
}

// setupFlags はコマンドライン引数の定義とデフォルト値の設定を行います。
// cmd パッケージ内で再利用されるユーティリティ関数として残します。
func setupFlags(flagSet *flag.FlagSet, cfg *ReviewConfig, isBacklogContext bool) {
	// --- 必須の引数 ---
	flagSet.StringVar(&cfg.GitCloneURL, "git-clone-url", "",
		"レビュー対象のGitリポジトリURL")
	flagSet.StringVar(&cfg.BaseBranch, "base-branch", "",
		"差分比較の基準ブランチ")
	flagSet.StringVar(&cfg.FeatureBranch, "feature-branch", "",
		"レビュー対象のフィーチャーブランチ")

	// --- 任意の引数 ---

	// local-pathのデフォルト値処理
	defaultLocalPath := filepath.Join(os.TempDir(), "git-reviewer-repos", "tmp")
	flagSet.StringVar(&cfg.LocalPath, "local-path", defaultLocalPath,
		fmt.Sprintf("リポジトリを格納するローカルパス (デフォルト: %s)", defaultLocalPath))

	issueHelp := "関連する課題ID (レビュープロンプトのコンテキストに使用)"
	if isBacklogContext {
		issueHelp += " (Backlog投稿時には必須)"
	}
	flagSet.StringVar(&cfg.IssueID, "issue-id", "", issueHelp)

	flagSet.StringVar(&cfg.GeminiModelName, "gemini-model-name", "gemini-2.5-flash",
		"使用するGeminiモデル名 (デフォルト: gemini-2.5-flash)")

	// Backlog投稿スキップフラグ
	if isBacklogContext {
		flagSet.BoolVar(&cfg.NoPost, "no-post", false,
			"レビュー結果をBacklogにコメント投稿するのをスキップします。")
	}
}

// validateRequiredArgs は必須引数が設定されているかチェックします。
// cmd パッケージ内で再利用されるユーティリティ関数として残します。
func validateRequiredArgs(cfg *ReviewConfig, flagSet *flag.FlagSet) bool {
	valid := true

	// 必須引数チェックのリスト
	required := map[string]string{
		"git-clone-url":  cfg.GitCloneURL,
		"base-branch":    cfg.BaseBranch,
		"feature-branch": cfg.FeatureBranch,
	}

	for name, value := range required {
		if value == "" {
			fmt.Fprintf(os.Stderr, "エラー: 必須引数 -%s が指定されていません。\n", name)
			valid = false
		}
	}

	if !valid {
		fmt.Fprintln(os.Stderr, "\n使用方法:")
		flagSet.PrintDefaults()
	}

	return valid
}

// main はプログラムのエントリポイントです。
func main() {
	// 全ての CLI ロジックを cmd パッケージに委譲します。
	cmd.Execute()
}
