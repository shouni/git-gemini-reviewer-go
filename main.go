package main

import (
	"flag"
	"fmt"
	"git-gemini-reviewer-go/cmd"
	"os"
	"path/filepath"
) // 👈 これで十分

// ReviewConfig はコマンドライン引数を保持する構造体です。
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
// 💡 修正点: flagSet を引数として受け取り、そのメソッドでフラグをバインドします。
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

// 実行するモードを定義
const (
	ModeBacklog = "backlog"
	ModeGeneric = "generic"
)

func main() {
	cmd.Execute()
}

// runBacklogReviewer は Backlog 連携モードでの実行を処理します。
func runBacklogReviewer(args []string) {
	cfg := ReviewConfig{}

	flagSet := flag.NewFlagSet("backlog-reviewer", flag.ExitOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), "使用法: backlog-reviewer [OPTIONS]\n")
		fmt.Fprintln(flagSet.Output(), "Gitリポジトリの差分をGeminiでコードレビューし、Backlogにコメントします。")
		fmt.Fprintln(flagSet.Output(), "\nオプション:")
		flagSet.PrintDefaults()
	}

	// 💡 修正点: flagSetを渡す
	setupFlags(flagSet, &cfg, true)

	fullArgs := append([]string{"backlog-reviewer"}, args...)

	flagSet.Parse(fullArgs)

	if !validateRequiredArgs(&cfg, flagSet) {
		os.Exit(1)
	}

	// ... (Backlog投稿ロジックの再現) ...

	fmt.Printf("Backlogモードでレビューを実行します:\n%+v\n", cfg)

	os.Exit(0)
}

// runGenericReviewer は 汎用レビューモードでの実行を処理します。
func runGenericReviewer(args []string) {
	cfg := ReviewConfig{}

	flagSet := flag.NewFlagSet("git-gemini-review", flag.ExitOnError)
	flagSet.Usage = func() {
		fmt.Fprintf(flagSet.Output(), "使用法: git-gemini-review [OPTIONS]\n")
		fmt.Fprintln(flagSet.Output(), "Gitリポジトリの差分をGeminiでコードレビューし、結果を標準出力します。")
		fmt.Fprintln(flagSet.Output(), "\nオプション:")
		flagSet.PrintDefaults()
	}

	// 💡 修正点: flagSetを渡す
	setupFlags(flagSet, &cfg, false)

	fullArgs := append([]string{"git-gemini-review"}, args...)

	flagSet.Parse(fullArgs)

	if !validateRequiredArgs(&cfg, flagSet) {
		os.Exit(1)
	}

	// 汎用レビュークラスを呼び出すロジックをここに実装
	// 最終的にこの行が出力されれば成功です。
	fmt.Printf("汎用モードでレビューを実行します:\n%+v\n", cfg)

	os.Exit(0)
}
