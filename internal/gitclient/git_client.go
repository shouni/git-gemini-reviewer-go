package gitclient

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
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

// Service はGitリポジトリ操作の抽象化を提供します。
// (旧: GitService)
type Service interface {
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

// Client は Service インターフェースを実装する具体的な構造体です。
type Client struct {
	LocalPath                string
	SSHKeyPath               string
	BaseBranch               string
	InsecureSkipHostKeyCheck bool
	auth                     transport.AuthMethod
}

// Option はClientの初期化オプションを設定するための関数です。
type Option func(*Client)

// WithInsecureSkipHostKeyCheck はSSHホストキーチェックをスキップするオプションを設定します。
func WithInsecureSkipHostKeyCheck(skip bool) Option {
	return func(gc *Client) {
		gc.InsecureSkipHostKeyCheck = skip
	}
}

// WithBaseBranch はベースブランチを設定するオプションです。
func WithBaseBranch(branch string) Option {
	return func(gc *Client) {
		gc.BaseBranch = branch
	}
}

// NewClient はClientを初期化します。
// Serviceインターフェースを返します。
func NewClient(localPath string, sshKeyPath string, opts ...Option) Service {
	client := &Client{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// Cleanup は処理後にローカルリポジトリをクリーンな状態に戻します。
func (c *Client) Cleanup(repo *git.Repository) error { // レシーバー名を (c *Client) に変更
	// 修正: ログメッセージを日本語に
	slog.Info("クリーンアップ: ベースブランチへのチェックアウトを開始します。", "base_branch", c.BaseBranch)

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
	}

	// ローカルの状態を破棄し、BaseBranchにチェックアウトする
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(c.BaseBranch),
		Force:  true,
	})

	if err != nil {
		return fmt.Errorf("ベースブランチ '%s' へのチェックアウトに失敗しました: %w", c.BaseBranch, err)
	}

	// 修正: ログメッセージを日本語に
	slog.Info("クリーンアップ: ローカルリポジトリをベースブランチにリセットしました。", "base_branch", c.BaseBranch)
	return nil
}

// expandTilde はクロスプラットフォームなチルダ展開をサポートする
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("現在のユーザーのホームディレクトリの取得に失敗しました: %w", err)
	}
	return filepath.Join(currentUser.HomeDir, path[2:]), nil
}

// getAuthMethod は go-git が使用する認証方法を返します。
func (c *Client) getAuthMethod(repoURL string) (transport.AuthMethod, error) { // レシーバー名を (c *Client) に変更
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {

		u, err := url.Parse(repoURL)
		if err != nil {
			return nil, fmt.Errorf("リポジトリURLのパースに失敗しました: %w", err)
		}
		username := "git"
		if u.User != nil {
			username = u.User.Username()
		}

		sshKeyPath, err := expandTilde(c.SSHKeyPath)
		if err != nil {
			return nil, fmt.Errorf("SSHキーパスの展開に失敗しました: %w", err)
		}

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

		// InsecureSkipHostKeyCheck の設定を適用
		if c.InsecureSkipHostKeyCheck {
			auth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
		} else {
			auth.HostKeyCallback = nil // known_hosts を使用
		}

		return auth, nil
	}
	return nil, nil
}

// cloneRepository は go-git.PlainClone を使用してクローン処理を実行するヘルパー関数です。
func (c *Client) cloneRepository(repositoryURL, localPath, branch string) error { // レシーバー名を (c *Client) に変更
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

	// 修正: ログメッセージを日本語に
	slog.Info("Go-gitを使用してリポジトリのクローンを開始します。", "url", repositoryURL, "path", localPath)

	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git クローン用の認証情報取得に失敗しました: %w", err)
	}

	_, err = git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Auth:          auth,
		Progress:      os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("go-git クローンに失敗しました: %w", err)
	}
	// 修正: ログメッセージを日本語に
	slog.Info("Go-gitによるリポジトリのクローンに成功しました。")
	return nil
}

// recloneRepository は、既存リポジトリを削除し、再クローンします。（修正 1のヘルパー）
func (c *Client) recloneRepository(repositoryURL, localPath, branch string) (*git.Repository, error) { // レシーバー名を (c *Client) に変更
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
		}
		// 修正: ログメッセージを日本語に
		slog.Info("再クローンのため、既存のリポジトリディレクトリを削除しました。", "path", localPath)
	}

	if err := c.cloneRepository(repositoryURL, localPath, branch); err != nil {
		return nil, fmt.Errorf("リポジトリのクローンに失敗しました: %w", err)
	}

	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("クローン後のリポジトリのオープンに失敗しました: %w", err)
	}
	return repo, nil
}

// updateExistingRepository は、既存リポジトリをプルで更新し、失敗した場合は再クローンが必要なエラーを返します。（修正 1のヘルパー）
func (c *Client) updateExistingRepository(repo *git.Repository, repositoryURL string) error { // レシーバー名を (c *Client) に変更
	authForPull, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git pull用の認証情報取得に失敗しました: %w", err)
	}

	// 修正: ログメッセージを日本語に
	slog.Info("リポジトリが既に存在します。go-git pullで更新します。", "path", c.LocalPath)
	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
	}

	pullErr := w.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(c.BaseBranch),
		Auth:          authForPull,
		SingleBranch:  true,
	})

	if pullErr == nil || pullErr == git.NoErrAlreadyUpToDate {
		// 修正: ログメッセージを日本語に
		slog.Info("go-git pullによるリポジトリの更新に成功しました。")
		return nil
	}

	// pull失敗時のリカバリロジック
	// 修正: ログメッセージを日本語に
	slog.Info("警告: go-git pullに失敗しました。リカバリのために再クローンを試行します。", "error", pullErr)
	// 再クローンのためにディレクトリを削除
	if err := os.RemoveAll(c.LocalPath); err != nil {
		return fmt.Errorf("pull失敗後の既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", c.LocalPath, err)
	}

	// pull失敗により、再クローンが必要であることを示すエラーを返す
	return fmt.Errorf("pull failed, reclone required: %w", pullErr)
}

// CloneOrUpdate はリポジトリをクローンするか、既に存在する場合は go-git pull で更新します。
func (c *Client) CloneOrUpdate(repositoryURL string) (*git.Repository, error) { // レシーバー名を (c *Client) に変更
	localPath := c.LocalPath
	var repo *git.Repository
	var err error

	if c.repoNeedsReclone(repositoryURL, localPath) {
		// 修正: ログメッセージを日本語に
		slog.Info("指定されたリポジトリまたはURLと異なるため、クローンまたは再クローンが必要です。", "path", localPath, "url", repositoryURL)
		repo, err = c.recloneRepository(repositoryURL, localPath, c.BaseBranch)
		if err != nil {
			return nil, err
		}
	} else {
		// 既存リポジトリのオープン
		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return nil, fmt.Errorf("既存リポジトリのオープンに失敗しました: %w", err)
		}

		// プルとリカバリ
		if pullErr := c.updateExistingRepository(repo, repositoryURL); pullErr != nil {
			// updateExistingRepositoryがエラーを返した場合、再クローンが必要か判断
			if strings.HasPrefix(pullErr.Error(), "pull failed, reclone required") {
				// 修正: ログメッセージを日本語に
				slog.Info("リカバリのための再クローンを開始します...")
				// 再クローン
				repo, err = c.recloneRepository(repositoryURL, localPath, c.BaseBranch)
				if err != nil {
					return nil, err
				}
			} else {
				// pull自体が致命的なエラーだった場合 (認証失敗など)
				return nil, pullErr
			}
		}
	}
	// go-git および Fetchで認証情報を使えるよう、最後にc.authを設定する
	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	c.auth = auth // Clientインスタンスに認証情報を保持
	// 修正: ログメッセージを日本語に
	slog.Info("Go-git用の認証情報がクライアントに正常に設定されました。")

	return repo, nil
}

// Fetch はリモートから最新の変更を取得します。
func (c *Client) Fetch(repo *git.Repository) error { // レシーバー名を (c *Client) に変更
	// 修正: ログメッセージを日本語に
	slog.Info("リモートから最新の変更をフェッチしています...", "path", c.LocalPath)
	if c.auth == nil {
		return fmt.Errorf("認証情報が設定されていません。ClientのAuthMethodを設定するには、先にCloneOrUpdateを実行してください。")
	}

	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err := repo.Fetch(&git.FetchOptions{
		Auth:     c.auth, // CloneOrUpdateで設定された認証情報を使用
		RefSpecs: []config.RefSpec{refSpec},
		Progress: os.Stdout,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("リモートからのフェッチに失敗しました: %w", err)
	}

	return nil
}

// getTwoDotDiff は 2-dot diff (A..B) を計算するヘルパー
func (c *Client) getTwoDotDiff(baseCommit, featureCommit *object.Commit) (string, error) { // レシーバー名を (c *Client) に変更
	baseTree, err := baseCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("ベースツリー(2-dot)の取得に失敗しました: %w", err)
	}

	featureTree, err := featureCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("フィーチャーツリー(2-dot)の取得に失敗しました: %w", err)
	}

	changes, err := baseTree.Diff(featureTree)
	if err != nil {
		return "", fmt.Errorf("ツリーの差分取得(2-dot)に失敗しました: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("パッチの生成(2-dot)に失敗しました: %w", err)
	}

	return patch.String(), nil
}

// GetCodeDiff は指定された2つのブランチ間の純粋な差分を、go-gitのみで取得します。
func (c *Client) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) { // レシーバー名を (c *Client) に変更
	// 修正: ログメッセージを日本語に
	slog.Info("Go-gitを使用して差分を計算しています。", "path", c.LocalPath, "base_branch", baseBranch, "feature_branch", featureBranch)

	// 1. ブランチ参照を解決
	baseRefName := plumbing.NewRemoteReferenceName("origin", baseBranch)
	baseRef, err := repo.Reference(baseRefName, false)
	if err != nil {
		return "", fmt.Errorf("ベースブランチ '%s' の参照解決に失敗しました: %w", baseBranch, err)
	}

	featureRefName := plumbing.NewRemoteReferenceName("origin", featureBranch)
	featureRef, err := repo.Reference(featureRefName, false)
	if err != nil {
		return "", fmt.Errorf("フィーチャーブランチ '%s' の参照解決に失敗しました: %w", featureBranch, err)
	}

	// 2. コミットオブジェクトを取得
	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return "", fmt.Errorf("ベースコミット '%s' の取得に失敗しました: %w", baseRef.Hash(), err)
	}

	featureCommit, err := repo.CommitObject(featureRef.Hash())
	if err != nil {
		return "", fmt.Errorf("フィーチャーコミット '%s' の取得に失敗しました: %w", featureRef.Hash(), err)
	}

	// 3. マージベース(共通祖先)の検索 (git diff A...B のため)
	mergeBaseCommits, err := baseCommit.MergeBase(featureCommit)
	if err != nil {
		return "", fmt.Errorf("マージベースの検索に失敗しました: %w", err)
	}

	if len(mergeBaseCommits) == 0 {
		return "", fmt.Errorf("ブランチ '%s' と '%s' の間に共通の祖先が見つかりませんでした。3-dot diffは計算できません。", baseBranch, featureBranch)
	}

	mergeBaseCommit := mergeBaseCommits[0]

	// 4. ツリーの取得
	baseTree, err := mergeBaseCommit.Tree() // マージベースのツリー
	if err != nil {
		return "", fmt.Errorf("マージベースのツリー取得に失敗しました: %w", err)
	}

	featureTree, err := featureCommit.Tree() // フィーチャーブランチのツリー
	if err != nil {
		return "", fmt.Errorf("フィーチャーブランチのツリー取得に失敗しました: %w", err)
	}

	// 5. 差分 (Changes) の生成
	changes, err := baseTree.Diff(featureTree)
	if err != nil {
		return "", fmt.Errorf("ツリーの差分取得に失敗しました: %w", err)
	}

	// 6. Patch オブジェクトに変換
	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("パッチの生成に失敗しました: %w", err)
	}

	// 7. 文字列として返す
	return patch.String(), nil
}

// CheckRemoteBranchExists は指定されたブランチがリモート 'origin' に存在するか確認します。
func (c *Client) CheckRemoteBranchExists(repo *git.Repository, branch string) (bool, error) { // レシーバー名を (c *Client) に変更
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
func (c *Client) repoNeedsReclone(repositoryURL, localPath string) bool { // レシーバー名を (c *Client) に変更
	gitDir := filepath.Join(localPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// 修正: ログメッセージを日本語に
		slog.Info(".gitディレクトリが見つかりません。クローンが必要です。", "path", localPath)
		return true
	}
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		// 修正: ログメッセージを日本語に
		slog.Warn("既存のリポジトリを開けませんでした。再クローンを試行します。", "path", localPath, "error", err)
		return true
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		// 修正: ログメッセージを日本語に
		slog.Warn("既存のリポジトリにリモート 'origin' が見つかりません。再クローンを試行します。", "path", localPath, "error", err)
		return true
	}
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != repositoryURL {
		// 修正: ログメッセージを日本語に
		slog.Warn("既存リポジトリのリモートURLが要求されたURLと一致しません。再クローンを試行します。", "existing_urls", remoteURLs, "requested_url", repositoryURL)
		return true
	}
	return false
}
