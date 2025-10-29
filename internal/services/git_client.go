package services

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// GitService はGitリポジトリ操作の抽象化を提供します。
type GitService interface {
	// CloneOrUpdate はリポジトリをクローンまたは更新し、go-gitリポジトリオブジェクトを返します。
	CloneOrUpdate(repositoryURL string) (*git.Repository, error)
	// Fetch はリモートから最新の変更を取得します。
	Fetch(repo *git.Repository) error
	// CheckRemoteBranchExists は指定されたブランチがリモートに存在するか確認します。
	CheckRemoteBranchExists(repo *git.Repository, branch string) (bool, error)
	// GetCodeDiff は指定された2つのブランチ間の純粋な差分を文字列として取得します。
	GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error)
	// Cleanup は処理後にローカルリポジトリをクリーンな状態に戻します。
	Cleanup(repo *git.Repository) error
}

// GitClient は GitService インターフェースを実装する具体的な構造体です。
type GitClient struct {
	LocalPath  string
	SSHKeyPath string
	// 主にCloneOrUpdateやPullのデフォルトブランチとして使用されます。
	// GetCodeDiffの引数は個別の比較対象として扱われます。
	BaseBranch               string
	InsecureSkipHostKeyCheck bool
	auth                     transport.AuthMethod
}

// GitClientOption はGitClientの初期化オプションを設定するための関数です。
type GitClientOption func(*GitClient)

// WithInsecureSkipHostKeyCheck はSSHホストキーチェックをスキップするオプションを設定します。
func WithInsecureSkipHostKeyCheck(skip bool) GitClientOption {
	return func(gc *GitClient) {
		gc.InsecureSkipHostKeyCheck = skip
	}
}

// WithBaseBranch はベースブランチを設定するオプションです。
func WithBaseBranch(branch string) GitClientOption {
	return func(gc *GitClient) {
		gc.BaseBranch = branch
	}
}

// NewGitClient はGitClientを初期化します。
// BaseBranchはオプションで設定されることを想定し、引数から削除されました。
// GitServiceインターフェースを返します。
func NewGitClient(localPath string, sshKeyPath string, opts ...GitClientOption) GitService {
	client := &GitClient{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// Cleanup は処理後にローカルリポジトリをクリーンな状態に戻します。
// go-gitのWorktreeをBaseBranchにチェックアウトし、ローカルの状態をリセットします。
func (c *GitClient) Cleanup(repo *git.Repository) error {
	// ログを出力
	log.Printf("Cleanup: Checking out base branch '%s'...\n", c.BaseBranch)

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("Cleanup: ワークツリーの取得に失敗しました: %w", err)
	}

	// ローカルの状態を破棄し、BaseBranchにチェックアウトする
	// 強制的に切り替える (git checkout -f <branch>)
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(c.BaseBranch),
		Force:  true,
	})

	if err != nil {
		return fmt.Errorf("Cleanup: ベースブランチ '%s' へのチェックアウトに失敗しました: %w", c.BaseBranch, err)
	}

	log.Printf("Cleanup: Local repository successfully reset to base branch '%s'.\n", c.BaseBranch)
	return nil
}

// expandTilde はクロスプラットフォームなチルダ展開をサポートする
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	currentUser, err := user.Current()
	if err != nil {
		log.Printf("Warning: Failed to get current user home directory: %v. Using original path.\n", err)
		return path
	}
	return filepath.Join(currentUser.HomeDir, path[2:])
}

// getAuthMethod は go-git が使用する認証方法を返します。
func (c *GitClient) getAuthMethod(repoURL string) (transport.AuthMethod, error) {
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {

		u, err := url.Parse(repoURL)
		if err != nil {
			return nil, fmt.Errorf("リポジトリURLのパースに失敗しました: %w", err)
		}
		username := "git"
		if u.User != nil {
			username = u.User.Username()
		}

		sshKeyPath := expandTilde(c.SSHKeyPath)

		if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SSHキーファイルが見つかりません: %s", sshKeyPath)
		}

		sshKey, err := os.ReadFile(sshKeyPath)
		if err != nil {
			return nil, fmt.Errorf("SSHキーファイルの読み込みに失敗しました: %w", err)
		}

		auth, err := ssh.NewPublicKeys(username, sshKey, "")
		if err != nil {
			return nil, fmt.Errorf("SSH認証キーのロードに失敗しました: %w", err)
		}

		// InsecureSkipHostKeyCheck の設定を適用 (go-gitの内部SSHクライアントに適用)
		if c.InsecureSkipHostKeyCheck {
			auth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
		} else {
			// ホストキーチェックを有効にする (known_hosts を使用)
			auth.HostKeyCallback = nil
		}

		return auth, nil
	}
	return nil, nil
}

// getGitSSHCommand は外部の 'git' コマンドで使用するための GIT_SSH_COMMAND 環境変数の値を構築します。
func (c *GitClient) getGitSSHCommand() (string, error) {
	if c.SSHKeyPath == "" {
		return "", nil
	}

	sshKeyPath := expandTilde(c.SSHKeyPath)

	if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SSHキーファイルが見つかりません: %s", sshKeyPath)
	}

	// HostKeyCheckingを無効化するオプションと、秘密鍵のパスを指定。
	// クロスプラットフォーム互換性のため、パスをToSlashで変換します。
	// -F /dev/null はシステム設定を無視し、環境変数のオプションを優先させる。
	cmd := fmt.Sprintf("ssh -i %s -F /dev/null", filepath.ToSlash(sshKeyPath))

	if c.InsecureSkipHostKeyCheck {
		cmd += " -o StrictHostKeyChecking=no"
	}

	return cmd, nil
}

// cloneRepository は go-git.PlainClone を使用してクローン処理を実行するヘルパー関数です。
func (c *GitClient) cloneRepository(repositoryURL, localPath, branch string) error {
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

	log.Printf("Cloning %s into %s using go-git...\n", repositoryURL, localPath)

	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git クローン用の認証情報取得に失敗しました: %w", err)
	}

	_, err = git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Auth:          auth,
		// Progressオプションは進捗をos.Stdoutに出力します。
		// より制御されたログ出力のためには、カスタムのio.Writerを実装するか、log.Writer()を使用することを検討してください。
		Progress: os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("go-git クローンに失敗しました: %w", err)
	}
	log.Println("Repository cloned successfully using go-git.")
	return nil
}

// CloneOrUpdate はリポジトリをクローンするか、既に存在する場合は go-git pull で更新します。
func (c *GitClient) CloneOrUpdate(repositoryURL string) (*git.Repository, error) {

	var err error
	var repo *git.Repository

	localPath := c.LocalPath

	if c.repoNeedsReclone(repositoryURL, localPath) {
		log.Printf("Repository at %s needs to be cloned or re-cloned for %s.\n", localPath, repositoryURL)
		if _, err := os.Stat(localPath); err == nil {
			if err := os.RemoveAll(localPath); err != nil {
				return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
			}
			log.Printf("Existing repository at %s removed.\n", localPath)
		}

		if err := c.cloneRepository(repositoryURL, localPath, c.BaseBranch); err != nil {
			return nil, fmt.Errorf("リポジトリのクローンに失敗しました: %w", err)
		}

		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return nil, fmt.Errorf("クローン後のリポジトリのオープンに失敗しました: %w", err)
		}

	} else {
		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return nil, fmt.Errorf("既存リポジトリのオープンに失敗しました: %w", err)
		}

		authForPull, err := c.getAuthMethod(repositoryURL)
		if err != nil {
			return nil, fmt.Errorf("go-git pull用の認証情報取得に失敗しました: %w", err)
		}

		log.Printf("Repository already exists at %s with matching URL. Running 'go-git pull' to update...\n", localPath)
		w, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
		}

		pullErr := w.Pull(&git.PullOptions{
			RemoteName:    "origin",
			ReferenceName: plumbing.NewBranchReferenceName(c.BaseBranch),
			Auth:          authForPull,
			SingleBranch:  true,
		})

		if pullErr == nil || pullErr == git.NoErrAlreadyUpToDate {
			log.Println("Repository updated successfully using go-git pull.")
		} else {
			log.Printf("Warning: go-git pull failed: %v. Attempting to recover by re-cloning...", pullErr)

			// pull失敗時にローカルの状態をクリーンアップし、リモートに強制的に合わせる
			// レビューツールとしては、常にクリーンな状態から開始することが重要であるため、この戦略を採用
			w.Reset(&git.ResetOptions{Mode: git.HardReset})

			if err := os.RemoveAll(localPath); err != nil {
				return nil, fmt.Errorf("pull失敗後の既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
			}
			log.Printf("Existing repository at %s removed for re-cloning.\n", localPath)

			if err := c.cloneRepository(repositoryURL, localPath, c.BaseBranch); err != nil {
				return nil, fmt.Errorf("pull失敗後の再クローンに失敗しました: %w", err)
			}
			log.Println("Repository re-cloned successfully.")

			repo, err = git.PlainOpen(localPath)
			if err != nil {
				return nil, fmt.Errorf("再クローン後のリポジトリのオープンに失敗しました: %w", err)
			}
		}
	}

	// go-git および Fetchで認証情報を使えるよう、最後にc.authを設定する
	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	c.auth = auth
	log.Println("go-git authentication method has been set successfully.")

	return repo, nil
}

// Fetch はリモートから最新の変更を取得します。
func (c *GitClient) Fetch(repo *git.Repository) error {
	log.Printf("Fetching latest changes from remote for repository at %s...\n", c.LocalPath)

	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err := repo.Fetch(&git.FetchOptions{
		Auth:     c.auth, // CloneOrUpdateで設定された認証情報を使用
		RefSpecs: []config.RefSpec{refSpec},
		Progress: os.Stdout,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to fetch from remote: %w", err)
	}

	return nil
}

// GetCodeDiff は指定された2つのブランチ間の純粋な差分を、外部コマンドで高速に取得します。
func (c *GitClient) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	// NOTE: 大規模リポジトリでのパフォーマンス問題を回避するため、go-gitのPatchメソッドではなく、外部の 'git diff' コマンドを使用している。
	log.Printf("Calculating code diff for repository at %s between remote/%s and remote/%s using external 'git diff' command...\n", c.LocalPath, baseBranch, featureBranch)

	// コマンド引数: git diff --no-color origin/<base>...origin/<feature> (3点比較)
	cmdArgs := []string{
		"diff",
		"--no-color", // 色付けを無効にする
		fmt.Sprintf("origin/%s...origin/%s", baseBranch, featureBranch),
	}

	cmd := exec.Command("git", cmdArgs...)
	cmd.Dir = c.LocalPath // リポジトリのローカルパスで実行

	// 最小限の環境変数（PATH, HOMEなど）をコピーし、GIT_SSH_COMMANDを設定/上書きする
	env := os.Environ()

	gitSSHCommand, err := c.getGitSSHCommand()
	if err != nil {
		return "", err
	}

	if gitSSHCommand != "" {
		found := false
		for i, e := range env {
			if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
				env[i] = fmt.Sprintf("GIT_SSH_COMMAND=%s", gitSSHCommand)
				found = true
				break
			}
		}
		if !found {
			env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", gitSSHCommand))
		}
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git diff 実行に失敗しました: %w. Stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// CheckRemoteBranchExists は指定されたブランチがリモート 'origin' に存在するか確認します。
func (c *GitClient) CheckRemoteBranchExists(repo *git.Repository, branch string) (bool, error) {
	if branch == "" {
		return false, fmt.Errorf("リモートブランチの存在確認に失敗しました: ブランチ名が空です")
	}
	refName := plumbing.NewRemoteReferenceName("origin", branch)

	_, err := repo.Reference(refName, false)

	if err == plumbing.ErrReferenceNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("リモートブランチ '%s' の確認に失敗しました: %w", branch, err)
	}

	return true, nil
}

// repoNeedsReclone はリポジトリを再クローンする必要があるかをチェックするヘルパー関数
func (c *GitClient) repoNeedsReclone(repositoryURL, localPath string) bool {
	gitDir := filepath.Join(localPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		log.Printf("Info: .git directory not found at %s. Cloning needed.\n", localPath)
		return true
	}
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		log.Printf("Warning: Existing repository at %s could not be opened: %v. Re-cloning...\n", localPath, err)
		return true
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		log.Printf("Warning: Remote 'origin' not found in %s: %v. Re-cloning...\n", localPath, err)
		return true
	}
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != repositoryURL {
		log.Printf("Warning: Existing repository remote URL (%v) does not match the requested URL (%s). Re-cloning...\n", remoteURLs, repositoryURL)
		return true
	}
	return false
}
