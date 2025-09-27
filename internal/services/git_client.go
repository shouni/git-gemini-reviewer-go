package services

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitClient はGitリポジトリ操作を管理します。
type GitClient struct {
	LocalPath  string
	SSHKeyPath string // SSHキーファイルのパス
}

// NewGitClient はGitClientを初期化します。
func NewGitClient(localPath string, sshKeyPath string) *GitClient {
	return &GitClient{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}
}

// expandTilde はパス内のチルダ (~) をホームディレクトリに展開します。
func expandTilde(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		// "~/" をホームディレクトリパスで置き換える
		return filepath.Join(usr.HomeDir, path[2:]), nil
	}
	return path, nil
}

// getAuthMethod はSSHキーファイルから認証メソッドを作成します。
func (c *GitClient) getAuthMethod() (transport.AuthMethod, error) {
	if c.SSHKeyPath == "" {
		// キーパスが指定されていない場合は、認証なし (パブリックリポジトリ用)
		return nil, nil
	}

	// 💡 修正: パスを使用する前にチルダを展開する
	keyPath, err := expandTilde(c.SSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand SSH key path: %w", err)
	}

	// 秘密鍵のパスと、必要であればパスフレーズを指定
	auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		// ⚠️ 注意: デフォルトパスでファイルが存在しない場合もエラーになります
		//       ただし、存在しないパスを許可すると意図しない認証なしになってしまうため、
		//       エラーとして通知するのが望ましいです。
		return nil, fmt.Errorf("failed to create SSH public keys from %s: %w", keyPath, err)
	}
	return auth, nil
}

// CloneOrOpen はリポジトリをクローンするか、既に存在する場合は開きます。
func (c *GitClient) CloneOrOpen(url string) (*git.Repository, error) {
	auth, err := c.getAuthMethod()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(c.LocalPath); os.IsNotExist(err) {
		// ディレクトリが存在しない場合はクローン
		fmt.Printf("Cloning %s into %s...\n", url, c.LocalPath)
		repo, err := git.PlainClone(c.LocalPath, false, &git.CloneOptions{
			URL:      url,
			Auth:     auth, // 💡 認証情報を適用
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
		return repo, nil
	}

	// 既に存在する場合は開く
	fmt.Printf("Opening repository at %s...\n", c.LocalPath)
	repo, err := git.PlainOpen(c.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing repository: %w", err)
	}
	return repo, nil
}

// Fetch はリモートから最新のブランチ情報を取得します。
func (c *GitClient) Fetch(repo *git.Repository) error {
	auth, err := c.getAuthMethod()
	if err != nil {
		return err
	}

	fmt.Println("Fetching latest changes from remote...")

	// リモートトラッキングブランチの更新を保証するためのRefSpec
	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err = repo.Fetch(&git.FetchOptions{
		Auth:     auth, // 💡 認証情報を適用
		RefSpecs: []config.RefSpec{refSpec},
		Progress: os.Stdout,
	})

	// エラーが nil かつ "already up-to-date" でもない場合のみ、エラーを返す
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}
	return nil
}

// GetCodeDiff は指定された2つのリモートブランチ間の差分を取得します。
func (c *GitClient) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	w, err := repo.Worktree()
	if err != nil {
		// リポジトリがベアでないことを確認するため
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	_ = w

	// 1. ベースブランチのコミットを取得 (リモートトラッキング参照を使用)
	baseRef := plumbing.Revision(fmt.Sprintf("refs/remotes/origin/%s", baseBranch))
	baseCommitHash, err := repo.ResolveRevision(baseRef)
	if err != nil {
		return "", fmt.Errorf("base branch '%s' not found: %w", baseBranch, err)
	}
	baseCommit, err := repo.CommitObject(*baseCommitHash)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit: %w", err)
	}

	// 2. フィーチャーブランチのコミットを取得 (リモートトラッキング参照を使用)
	featureRef := plumbing.Revision(fmt.Sprintf("refs/remotes/origin/%s", featureBranch))
	featureCommitHash, err := repo.ResolveRevision(featureRef)
	if err != nil {
		return "", fmt.Errorf("feature branch '%s' not found: %w", featureBranch, err)
	}
	featureCommit, err := repo.CommitObject(*featureCommitHash)
	if err != nil {
		return "", fmt.Errorf("failed to get feature commit: %w", err)
	}

	// 3. 差分を取得
	patch, err := baseCommit.Patch(featureCommit)
	if err != nil {
		return "", fmt.Errorf("failed to generate patch (diff): %w", err)
	}

	return patch.String(), nil
}
