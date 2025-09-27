// internal/services/git_client.go
package services

import (
	"fmt"
	"os/exec"
)

// GitManager は Git操作を担当するサービス
type GitManager struct {
	LocalPath string
}

// NewGitManager は GitManager の新しいインスタンスを作成
func NewGitManager(localPath string) *GitManager {
	return &GitManager{LocalPath: localPath}
}

// GetDiff は指定された2つのブランチ間の差分（Diff）を取得します。
func (m *GitManager) GetDiff(baseBranch, featureBranch string) (string, error) {
	// git diff baseBranch...featureBranch コマンドを構築
	diffRange := fmt.Sprintf("%s...%s", baseBranch, featureBranch)

	// コマンド実行: git diff base...feature
	cmd := exec.Command("git", "diff", diffRange)
	cmd.Dir = m.LocalPath // コマンドを実行するディレクトリを指定

	// コマンドを実行し、標準出力と標準エラー出力を取得
	output, err := cmd.CombinedOutput()

	outputStr := string(output)

	// exec.Commandがエラーを返した場合（git diffが失敗した、gitが見つからないなど）
	if err != nil {
		return "", fmt.Errorf("git diff 実行失敗: %w\n詳細: %s", err, outputStr)
	}

	// 差分が空の場合のチェック（何も変更がない場合）
	if len(outputStr) == 0 {
		return "", fmt.Errorf("エラー: ブランチ %s と %s の間に差分が見つかりません。", baseBranch, featureBranch)
	}

	return outputStr, nil
}
