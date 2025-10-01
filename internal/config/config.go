// internal/config/config.go

package config

// ReviewConfig はGit操作とGeminiレビューの実行に必要な共通の設定を保持します。
type ReviewConfig struct {
	GitCloneURL     string
	BaseBranch      string
	FeatureBranch   string
	LocalPath       string
	GeminiModelName string
	SSHKeyPath      string
	PromptFilePath  string
	InsecureSkipHostKeyCheck bool
}
