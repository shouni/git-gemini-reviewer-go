package services

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

// GitClient はGitリポジトリ操作を管理します。
type GitClient struct {
	LocalPath                string
	SSHKeyPath               string
	BaseBranch               string
	auth                     transport.AuthMethod
	InsecureSkipHostKeyCheck bool
}

// NewGitClient はGitClientを初期化します。
func NewGitClient(localPath string, sshKeyPath string) *GitClient {
	return &GitClient{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}
}

// expandTilde はクロスプラットフォームなチルダ展開をサポートする
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	currentUser, err := user.Current()
	if err != nil {
		// エラーハンドリング: チルダ展開に失敗した場合は元のパスを返すか、エラーをログに記録
		fmt.Fprintf(os.Stderr, "Warning: Failed to get current user home directory: %v. Using original path.\n", err)
		return path
	}
	return filepath.Join(currentUser.HomeDir, path[2:])
}

// getAuthMethod はリポジトリURLに基づいて適切な認証方法を返します。
// SSH URLの場合は、指定された鍵ファイルを直接読み込んで認証を設定します。
func (c *GitClient) getAuthMethod(repoURL string) (transport.AuthMethod, error) {
	// ↓↓↓ このデバッグログを追加して、新しいコードが実行されているか確認する ↓↓↓
	fmt.Println("DEBUG: getAuthMethod is called. Using direct key file reader method.")

	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {

		// 2. URLをパースしてユーザー名を取得する
		u, err := url.Parse(repoURL)
		if err != nil {
			return nil, fmt.Errorf("リポジトリURLのパースに失敗しました: %w", err)
		}
		username := "git" // デフォルトは "git"
		if u.User != nil {
			username = u.User.Username()
		}
		fmt.Printf("DEBUG: Using username '%s' for SSH authentication.\n", username)

		sshKeyPath := expandTilde(c.SSHKeyPath)

		if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("SSHキーファイルが見つかりません: %s", sshKeyPath)
		}

		// 秘密鍵ファイルを直接読み込む
		sshKey, err := os.ReadFile(sshKeyPath)
		if err != nil {
			return nil, fmt.Errorf("SSHキーファイルの読み込みに失敗しました: %w", err)
		}

		// 読み込んだ鍵データから認証情報を作成する
		auth, err := ssh.NewPublicKeys(username, sshKey, "") // ハードコードされた "git" を username に変更
		if err != nil {
			return nil, fmt.Errorf("SSH認証キーのロードに失敗しました: %w", err)
		}

		auth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()

		return auth, nil
	}
	return nil, nil
}

// getGitSSHCommand は、外部gitコマンドで使用するための GIT_SSH_COMMAND の値を返します。
// SSHキーの存在チェックと StrictHostKeyChecking=no オプションの設定を行います。
func (c *GitClient) getGitSSHCommand() (string, error) {
	sshKeyPath := expandTilde(c.SSHKeyPath)

	// SSHキーパスを絶対パスに解決
	absSSHKeyPath, err := filepath.Abs(sshKeyPath)
	if err != nil {
		return "", fmt.Errorf("SSHキーパスの解決に失敗しました: %w", err)
	}

	if _, err := os.Stat(absSSHKeyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("SSHキーファイルが見つかりません: %s", absSSHKeyPath)
	}

	// これにより、GIT_SSH_COMMAND経由で実行されるsshが、パスを正しく解釈できるようにする
	cleanPath := strings.ReplaceAll(absSSHKeyPath, "\\", "/")

	// ssh -i <鍵の絶対パス> ...
	sshCommand := fmt.Sprintf("ssh -i %s", cleanPath)
	// (上記のInsecureSkipHostKeyCheckのロジックをここに追加)
	if c.InsecureSkipHostKeyCheck {
		sshCommand += " -o StrictHostKeyChecking=no"
	}
	return sshCommand, nil
}

// クローン処理をカプセル化したヘルパー関数
func (c *GitClient) cloneRepository(repositoryURL, localPath, branch string, env []string) error {
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

	fmt.Printf("Cloning %s into %s...\n", repositoryURL, localPath)
	cmd := exec.Command("git", "clone", "--branch", branch, repositoryURL, localPath)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("git clone コマンドの実行に失敗しました: %w", runErr)
	}
	fmt.Println("Repository cloned successfully using exec.Command.")
	return nil
}

// CloneOrUpdateWithExec は、リポジトリをクローンするか、既に存在する場合は pull で更新します。
func (c *GitClient) CloneOrUpdateWithExec(repositoryURL string, localPath string) (*git.Repository, error) {

	// 1. GIT_SSH_COMMAND を設定
	gitSSHCommand, err := c.getGitSSHCommand()
	if err != nil {
		return nil, err
	}

	env := os.Environ()
	env = append(env, fmt.Sprintf("GIT_SSH_COMMAND=%s", gitSSHCommand))

	// リポジトリがクローン済みで、かつリモートURLが一致するかをチェックするヘルパー関数
	// trueを返す場合、再クローンが必要。falseの場合、pullで更新可能。
	repoNeedsReclone := func() bool {
		gitDir := filepath.Join(localPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			// .git ディレクトリが存在しないので、クローンが必要
			fmt.Printf("Info: .git directory not found at %s. Cloning needed.\n", localPath)
			return true
		}

		repo, err := git.PlainOpen(localPath)
		if err != nil {
			// リポジトリを開けない、または壊れている可能性があるので再クローン
			fmt.Printf("Warning: Existing repository at %s could not be opened: %v. Re-cloning...\n", localPath, err)
			return true
		}

		remote, err := repo.Remote("origin")
		if err != nil {
			// リモート'origin'がないので再クローン
			fmt.Printf("Warning: Remote 'origin' not found in %s: %v. Re-cloning...\n", localPath, err)
			return true
		}

		remoteURLs := remote.Config().URLs
		if len(remoteURLs) == 0 || remoteURLs[0] != repositoryURL {
			// リモートURLが一致しないので再クローン
			fmt.Printf("Warning: Existing repository remote URL (%v) does not match the requested URL (%s). Re-cloning...\n", remoteURLs, repositoryURL)
			return true
		}

		return false // 再クローンは不要、pullで更新可能
	}

	if repoNeedsReclone() {
		fmt.Printf("Repository at %s needs to be cloned or re-cloned for %s.\n", localPath, repositoryURL)

		// 既存のディレクトリが存在する場合のみ削除を試みる
		if _, err := os.Stat(localPath); err == nil {
			if err := os.RemoveAll(localPath); err != nil {
				return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
			}
			fmt.Printf("Existing repository at %s removed.\n", localPath)
		}

		// クローン実行
		if err := c.cloneRepository(repositoryURL, localPath, c.BaseBranch, env); err != nil {
			return nil, fmt.Errorf("リポジトリのクローンに失敗しました: %w", err)
		}
		fmt.Println("Repository cloned successfully using exec.Command.")

	} else {
		// リポジトリが存在し、リモートURLも一致するので pull で更新
		fmt.Printf("Repository already exists at %s with matching URL. Running 'git pull' to update...\n", localPath)
		branchToPull := c.BaseBranch
		cmd := exec.Command("git", "pull", "origin", branchToPull)
		cmd.Dir = localPath
		cmd.Env = env
		// `os.Stdout`, `os.Stderr` への直接出力については別途ログポリシーに合わせて修正
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git pull コマンドの実行に失敗しました: %w", err)
		}
		fmt.Println("Repository updated successfully using exec.Command.")
	}

	// 最終的に go-git でリポジトリを開いて返す
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("最終的なリポジトリのオープンに失敗しました: %w", err)
	}

	// 後続の go-git を使う処理 (Fetchなど) のために、認証情報を取得して構造体にセットする。
	// これが欠けていたことがエラーの根本原因。
	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	c.auth = auth
	fmt.Println("DEBUG: go-git authentication method has been set successfully.")

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
		ref, err := repo.Reference(refName, false)
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
