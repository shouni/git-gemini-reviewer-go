package gitclient

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// Service はGitリポジトリ操作の抽象化を提供します。
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
	auth                     transport.AuthMethod // auth.go で設定される認証メソッド
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

// CloneOrUpdate はリポジトリをクローンするか、既に存在する場合は go-git pull で更新します。
func (c *Client) CloneOrUpdate(repositoryURL string) (*git.Repository, error) {
	localPath := c.LocalPath
	var repo *git.Repository
	var err error

	if c.repoNeedsReclone(repositoryURL, localPath) {
		slog.Info("指定されたリポジトリまたはURLと異なるため、クローンまたは再クローンが必要です。", "path", localPath, "url", repositoryURL)
		repo, err = c.recloneRepository(repositoryURL, localPath, c.BaseBranch)
		if err != nil {
			return nil, err
		}
	} else {
		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return nil, fmt.Errorf("既存リポジトリのオープンに失敗しました: %w", err)
		}

		if pullErr := c.updateExistingRepository(repo, repositoryURL); pullErr != nil {
			if strings.HasPrefix(pullErr.Error(), "pull failed, reclone required") {
				slog.Info("リカバリのための再クローンを開始します...")
				repo, err = c.recloneRepository(repositoryURL, localPath, c.BaseBranch)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, pullErr
			}
		}
	}

	// 認証情報の取得と保持
	auth, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	c.auth = auth
	slog.Info("go-git用の認証情報がクライアントに正常に設定されました。")

	return repo, nil
}

// Fetch はリモートから最新の変更を取得します。
func (c *Client) Fetch(repo *git.Repository) error {
	slog.Info("リモートから最新の変更をフェッチしています...", "path", c.LocalPath)
	if c.auth == nil {
		return fmt.Errorf("認証情報が設定されていません。ClientのAuthMethodを設定するには、先にCloneOrUpdateを実行してください。")
	}

	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err := repo.Fetch(&git.FetchOptions{
		Auth:     c.auth,
		RefSpecs: []config.RefSpec{refSpec},
		Progress: io.Discard,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("リモートからのフェッチに失敗しました: %w", err)
	}

	return nil
}

// GetCodeDiff は指定された2つのブランチ間の純粋な差分を、go-gitのみで取得します。
func (c *Client) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	slog.Info("go-gitを使用して差分を計算しています。", "path", c.LocalPath, "base_branch", baseBranch, "feature_branch", featureBranch)

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
	baseTree, err := mergeBaseCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("マージベースのツリー取得に失敗しました: %w", err)
	}

	featureTree, err := featureCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("フィーチャーブランチのツリー取得に失敗しました: %w", err)
	}

	// 5. 差分 (Changes) の生成とパッチへの変換
	changes, err := baseTree.Diff(featureTree)
	if err != nil {
		return "", fmt.Errorf("ツリーの差分取得に失敗しました: %w", err)
	}

	patch, err := changes.Patch()
	if err != nil {
		return "", fmt.Errorf("パッチの生成に失敗しました: %w", err)
	}

	return patch.String(), nil
}

// CheckRemoteBranchExists は指定されたブランチがリモート 'origin' に存在するか確認します。
func (c *Client) CheckRemoteBranchExists(repo *git.Repository, branch string) (bool, error) {
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

// Cleanup は処理後にローカルリポジトリをクリーンな状態に戻します。
func (c *Client) Cleanup(repo *git.Repository) error {
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

	slog.Info("クリーンアップ: ローカルリポジトリをベースブランチにリセットしました。", "base_branch", c.BaseBranch)
	return nil
}
