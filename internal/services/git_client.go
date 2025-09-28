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
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("failed to get current user's home directory: %w", err)
		}
		// "~/" をホームディレクトリパスで置き換える
		return filepath.Join(usr.HomeDir, path[2:]), nil
	}
	return path, nil
}

// getAuthMethod はSSHキーファイルから認証メソッドを作成します。
// SSHKeyPathが空の場合、またはリポジトリURLがSSHでない場合はnilを返します。
func (c *GitClient) getAuthMethod(repoURL string) (transport.AuthMethod, error) {
	// 認証が不要な場合（HTTPSまたはSSHキーパスが未指定の場合）
	// go-gitは認証情報なしでもクローンを試みるため、キーパスが空の場合はnilを返します。
	if c.SSHKeyPath == "" || !strings.HasPrefix(repoURL, "git@") {
		return nil, nil
	}

	// 💡 パスを使用する前にチルダを展開する
	keyPath, err := expandTilde(c.SSHKeyPath)
	if err != nil {
		return nil, err
	}

	// 秘密鍵のパスと、必要であればパスフレーズを指定
	// ユーザー名 'git' はSSHプロトコルでの標準
	auth, err := ssh.NewPublicKeysFromFile("git", keyPath, "")
	if err != nil {
		// SSH認証が必要だが、キーファイルが見つからない、または読み込めない場合
		return nil, fmt.Errorf("failed to create SSH public keys from %s: %w", keyPath, err)
	}
	return auth, nil
}

// CloneOrOpen はリポジトリをクローンするか、既に存在する場合は開きます。
func (c *GitClient) CloneOrOpen(url string) (*git.Repository, error) {
	auth, err := c.getAuthMethod(url)
	if err != nil {
		return nil, err
	}

	// 1. クローン先ディレクトリが存在しない場合は、単純にクローン
	if _, err := os.Stat(c.LocalPath); os.IsNotExist(err) {
		fmt.Printf("Cloning %s into %s...\n", url, c.LocalPath)
		repo, err := git.PlainClone(c.LocalPath, false, &git.CloneOptions{
			URL:      url,
			Auth:     auth,
			Progress: os.Stdout,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to clone repository %s: %w", url, err)
		}
		return repo, nil
	}

	// 2. 既に存在する場合は開く
	fmt.Printf("Opening repository at %s...\n", c.LocalPath)
	repo, err := git.PlainOpen(c.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing repository at %s: %w", c.LocalPath, err)
	}

	// 3. 既存のリポジトリURLをチェックする
	remote, err := repo.Remote("origin")
	if err != nil {
		// リモート'origin'がない、またはエラーの場合、再クローンが安全
		fmt.Printf("Warning: Remote 'origin' not found or failed to read: %v. Re-cloning...\n", err)
		return c.recloneRepository(url, auth)
	}

	// Fetch URLを取得し、渡されたURLと一致するか確認
	// go-gitは複数のURLを格納する可能性があるため、最初のURLをチェック
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != url {
		// URLが一致しない場合、古いリポジトリなので削除してクローンし直す
		fmt.Printf("Warning: Existing repository remote URL (%s) does not match the requested URL (%s). Re-cloning...\n", remoteURLs[0], url)
		return c.recloneRepository(url, auth)
	}

	// 4. URLが一致する場合は、そのままリポジトリを返す
	return repo, nil
}

// recloneRepository は、既存のディレクトリを削除して新しいURLでクローンし直すヘルパー関数です。
func (c *GitClient) recloneRepository(url string, auth transport.AuthMethod) (*git.Repository, error) {
	// 既存のディレクトリを削除
	if err := os.RemoveAll(c.LocalPath); err != nil {
		return nil, fmt.Errorf("failed to remove old repository directory %s: %w", c.LocalPath, err)
	}

	// 新しいURLで再クローン
	fmt.Printf("Re-cloning %s into %s...\n", url, c.LocalPath)
	repo, err := git.PlainClone(c.LocalPath, false, &git.CloneOptions{
		URL:      url,
		Auth:     auth,
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s after cleanup: %w", url, err)
	}
	return repo, nil
}

// Fetch はリモートから最新のブランチ情報を取得します。
func (c *GitClient) Fetch(repo *git.Repository) error {
	// 認証メソッドを再利用
	auth, err := c.getAuthMethod("") // Fetch時はURLチェックをスキップするため空文字列を渡す
	if err != nil {
		return err
	}

	fmt.Println("Fetching latest changes from remote...")

	// リモートトラッキングブランチの更新を保証するためのRefSpec
	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err = repo.Fetch(&git.FetchOptions{
		Auth:     auth,
		RefSpecs: []config.RefSpec{refSpec},
		Progress: os.Stdout,
	})

	// エラーが nil かつ "already up-to-date" でもない場合のみ、エラーを返す
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	// 成功または既に最新の場合
	return nil
}

// GetCodeDiff は指定された2つのリモートブランチ間の差分を取得します。
// 'git diff baseBranch...featureBranch' に相当する差分を生成します。
func (c *GitClient) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	// Worktreeの取得は今回は不要なため削除（差分はコミットオブジェクト間で取得するため）

	// 1. ベースブランチのコミットを取得 (リモートトラッキング参照を使用)
	// 例: refs/remotes/origin/main
	baseRefName := fmt.Sprintf("refs/remotes/origin/%s", baseBranch)
	baseCommitHash, err := repo.ResolveRevision(plumbing.Revision(baseRefName))
	if err != nil {
		return "", fmt.Errorf("base branch ref '%s' not found: %w", baseRefName, err)
	}
	baseCommit, err := repo.CommitObject(*baseCommitHash)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit %s: %w", baseCommitHash.String(), err)
	}

	// 2. フィーチャーブランチのコミットを取得 (リモートトラッキング参照を使用)
	// 例: refs/remotes/origin/feature/new-feature
	featureRefName := fmt.Sprintf("refs/remotes/origin/%s", featureBranch)
	featureCommitHash, err := repo.ResolveRevision(plumbing.Revision(featureRefName))
	if err != nil {
		return "", fmt.Errorf("feature branch ref '%s' not found: %w", featureRefName, err)
	}
	featureCommit, err := repo.CommitObject(*featureCommitHash)
	if err != nil {
		return "", fmt.Errorf("failed to get feature commit %s: %w", featureCommitHash.String(), err)
	}

	// 3. 差分を取得
	// 💡 修正: 一般的な 'git diff base..feature' は featureCommit.Patch(baseCommit) の形で取得されます。
	// これは featureCommit にあるが baseCommit にはない変更を表します。
	patch, err := baseCommit.Patch(featureCommit) // baseCommit から featureCommit への変更
	if err != nil {
		return "", fmt.Errorf("failed to generate patch (diff) between %s and %s: %w", baseBranch, featureBranch, err)
	}

	return patch.String(), nil
}
