package services

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// GitClient はGitリポジトリ操作を管理します。
type GitClient struct {
	LocalPath  string
	SSHKeyPath string
	BaseBranch string
	auth       transport.AuthMethod
}

// NewGitClient はGitClientを初期化します。
func NewGitClient(localPath string, sshKeyPath string) *GitClient {
	return &GitClient{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}
}

// expandTilde はパスに含まれるチルダ(~)を展開します。
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

// getAuthMethod はリポジトリURLに基づいて適切な認証方法を返します。
// 現在はSSH URLの場合のみ鍵認証を設定します。
func (c *GitClient) getAuthMethod(repoURL string) (transport.AuthMethod, error) {
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {
		sshKeyPath := expandTilde(c.SSHKeyPath)
		if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SSHキーファイルが見つかりません: %s", sshKeyPath)
		}

		// 鍵認証の設定
		auth, err := ssh.NewPublicKeysFromFile("git", sshKeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("SSH認証キーのロードに失敗しました: %w", err)
		}
		return auth, nil
	}
	// HTTPSなど、認証不要な場合はnilを返す
	return nil, nil
}

// getGitSSHCommand は、外部gitコマンドで使用するための GIT_SSH_COMMAND の値を返します。
// SSHキーの存在チェックと StrictHostKeyChecking=no オプションの設定を行います。
func (c *GitClient) getGitSSHCommand() (string, error) {
	sshKeyPath := expandTilde(c.SSHKeyPath)

	// SSHキーファイルの存在チェック
	if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SSHキーファイルが見つかりません: %s", sshKeyPath)
	}

	// ssh -i <鍵のパス> -o StrictHostKeyChecking=no を設定
	gitSSHCommand := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", sshKeyPath)
	return gitSSHCommand, nil
}

// CloneOrUpdateWithExec は、リポジトリをクローンするか、既に存在する場合は pull で更新します。
func (c *GitClient) CloneOrUpdateWithExec(repositoryURL string, localPath string) error {

	// 1. GIT_SSH_COMMAND を設定
	gitSSHCommand, err := c.getGitSSHCommand()
	if err != nil {
		return err
	}

	// SSH環境変数を設定（後の git pull でも使用するため）
	env := os.Environ()
	env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", gitSSHCommand))

	// 2. クローン先ディレクトリの存在チェック
	_, err = os.Stat(localPath)
	repoExists := !os.IsNotExist(err)

	if repoExists {
		// 存在する: git pull を実行して更新する
		fmt.Printf("Repository already exists at %s. Running 'git pull' to update...\n", localPath)

		// 存在するリポジトリを開く（作業ディレクトリを localPath に変更）
		cmd := exec.Command("git", "pull", "origin", c.BaseBranch)
		cmd.Dir = localPath
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git pull コマンドの実行に失敗しました: %w", err)
		}
		fmt.Println("Repository updated successfully using exec.Command.")

	} else {
		// 存在しない: git clone を実行する
		fmt.Printf("Cloning %s into %s...\n", repositoryURL, localPath)

		// クローン先の親ディレクトリが存在しない場合は作成 (Cloneが失敗するのを防ぐ)
		parentDir := filepath.Dir(localPath)
		if _, err := os.Stat(parentDir); os.IsNotExist(err) {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
			}
		}

		// git clone コマンドを構築
		cmd := exec.Command(
			"git",
			"clone",
			repositoryURL,
			localPath,
		)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if runErr := cmd.Run(); runErr != nil {
			return fmt.Errorf("git clone コマンドの実行に失敗しました: %w", runErr)
		}
		fmt.Println("Repository cloned successfully using exec.Command.")
	}

	// 3. go-git でリポジトリを開く（URLチェックのため）
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		// クローン/プル後にリポジトリが開けないのは致命的
		return fmt.Errorf("クローン/更新後、リポジトリを開けませんでした: %w", err)
	}

	// 4. 既存のリポジトリURLをチェックする
	remote, err := repo.Remote("origin")
	if err != nil {
		// リモート'origin'がない、またはエラーの場合、再クローンが安全
		fmt.Printf("Warning: Remote 'origin' not found or failed to read: %v. Re-cloning...\n", err)

		// 【重要】recloneRepository は *git.Repository を返すため、ここではエラー処理のみを考慮
		if _, recloneErr := c.recloneRepository(repositoryURL); recloneErr != nil {
			return recloneErr
		}
		return nil
	}

	// Fetch URLを取得し、渡されたURLと一致するか確認
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != repositoryURL {
		// URLが一致しない場合、古いリポジトリなので削除してクローンし直す
		fmt.Printf("Warning: Existing repository remote URL (%s) does not match the requested URL (%s). Re-cloning...\n", remoteURLs[0], repositoryURL)

		// 【重要】再クローンし、エラーがあればそれを返す
		if _, recloneErr := c.recloneRepository(repositoryURL); recloneErr != nil {
			return recloneErr
		}
		return nil
	}

	// URLチェックOK
	return nil
}

// CloneOrOpen はリポジトリをクローンするか、既に存在する場合は開き、認証情報を保持します。（既存の go-git を利用した実装）
func (c *GitClient) CloneOrOpen(url string) (*git.Repository, error) {
	// 認証情報を取得し、GitClient構造体に保持
	auth, err := c.getAuthMethod(url)
	if err != nil {
		return nil, err
	}
	c.auth = auth

	// 1. クローン先ディレクトリが存在しない場合は、単純にクローン
	if _, err := os.Stat(c.LocalPath); os.IsNotExist(err) {
		fmt.Printf("Cloning %s into %s...\n", url, c.LocalPath)
		repo, err := git.PlainClone(c.LocalPath, false, &git.CloneOptions{
			URL:      url,
			Auth:     c.auth, // 保持した認証情報を使用
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
		return c.recloneRepository(url)
	}

	// Fetch URLを取得し、渡されたURLと一致するか確認
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != url {
		// URLが一致しない場合、古いリポジトリなので削除してクローンし直す
		fmt.Printf("Warning: Existing repository remote URL (%s) does not match the requested URL (%s). Re-cloning...\n", remoteURLs[0], url)
		return c.recloneRepository(url)
	}

	// 4. URLが一致する場合は、そのままリポジトリを返す
	return repo, nil
}

// recloneRepository は、既存のディレクトリを削除して新しいURLでクローンし直すヘルパー関数です。
func (c *GitClient) recloneRepository(url string) (*git.Repository, error) {
	// 既存のディレクトリを削除
	if err := os.RemoveAll(c.LocalPath); err != nil {
		return nil, fmt.Errorf("failed to remove old repository directory %s: %w", c.LocalPath, err)
	}

	// 新しいURLで再クローン
	fmt.Printf("Re-cloning %s into %s...\n", url, c.LocalPath)
	repo, err := git.PlainClone(c.LocalPath, false, &git.CloneOptions{
		URL:      url,
		Auth:     c.auth, // 保持した認証情報 c.auth を利用
		Progress: os.Stdout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone repository %s after cleanup: %w", url, err)
	}
	return repo, nil
}

// Fetch はリモートから最新の変更を取得します。
func (c *GitClient) Fetch(repo *git.Repository) error {
	fmt.Println("Fetching latest changes from remote...")

	// すべてのブランチのRefSpec
	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err := repo.Fetch(&git.FetchOptions{
		Auth:     c.auth, // 保持した認証情報を使用
		RefSpecs: []config.RefSpec{refSpec},
		Progress: os.Stdout,
	})

	// "already up-to-date" はエラーではないので無視
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	return nil
}

// GetCodeDiff は指定された2つのブランチ間の「純粋な差分」（3点比較）を、リモートの最新情報に基づいて取得します。
func (c *GitClient) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	const remoteName = "origin"

	// ヘルパー関数: リモートブランチのコミットオブジェクトを取得
	getRemoteCommit := func(branch string) (*object.Commit, error) {
		// 例: refs/remotes/origin/main
		refName := plumbing.NewRemoteReferenceName(remoteName, branch)
		ref, err := repo.Reference(refName, true)
		if err != nil {
			return nil, fmt.Errorf("リモートリファレンス '%s/%s' の取得に失敗しました: %w", remoteName, branch, err)
		}
		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return nil, fmt.Errorf("コミットオブジェクトの取得に失敗しました: %w", err)
		}
		return commit, nil
	}

	// 1. ベースブランチとフィーチャーブランチのコミットを取得
	baseCommit, err := getRemoteCommit(baseBranch)
	if err != nil {
		return "", fmt.Errorf("ベースブランチのコミット取得に失敗: %w", err)
	}

	featureCommit, err := getRemoteCommit(featureBranch)
	if err != nil {
		return "", fmt.Errorf("フィーチャーブランチのコミット取得に失敗: %w", err)
	}

	// --- 2. 3点比較のためのマージベースの特定 ---
	// 'git merge-base origin/base origin/feature' に相当する処理
	mergeBaseCommits, err := baseCommit.MergeBase(featureCommit)
	if err != nil {
		// go-gitのMergeBaseは、エラーを返さず空のスライスを返すケースが多いため、このエラーは通常、Git内部のエラー。
		return "", fmt.Errorf("マージベース計算中に内部エラーが発生しました: %w", err)
	}

	if len(mergeBaseCommits) == 0 {
		return "", fmt.Errorf("ベースブランチとフィーチャーブランチ間に共通の祖先コミット（マージベース）が見つかりませんでした")
	}

	// マージベースが複数ある場合でも、最初の一つを使用する
	mergeBaseCommit := mergeBaseCommits[0]

	// 3. パッチの生成 (3点比較の実現)
	// マージベースCommitからフィーチャーCommitへの差分を取得。
	// これは 'git diff <MergeBase> <feature>' と同義で、「純粋な差分」を抽出します。
	patch, err := mergeBaseCommit.Patch(featureCommit)
	if err != nil {
		return "", fmt.Errorf("差分パッチの生成に失敗しました: %w", err)
	}

	return patch.String(), nil
}
