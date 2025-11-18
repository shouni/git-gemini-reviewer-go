package adapters

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	// NOTE: getAuthMethod の定義があるパッケージをインポートする必要がありますが、
	// ここでは存在を前提とし、外部関数として扱います。
)

// GitService はGitリポジトリ操作の抽象化を提供します。
type GitService interface {
	// CloneOrUpdate はリポジトリをクローンまたは更新し、成功時に nil を返します。
	CloneOrUpdate(ctx context.Context, repositoryURL string) error
	// Fetch はリモートから最新の変更を取得します。
	Fetch(ctx context.Context) error
	// CheckRemoteBranchExists は指定されたブランチがリモートに存在するか確認します。
	CheckRemoteBranchExists(ctx context.Context, branch string) (bool, error)
	// GetCodeDiff は指定された2つのブランチ間の純粋な差分を文字列として取得します。
	GetCodeDiff(ctx context.Context, baseBranch, featureBranch string) (string, error)
	// Cleanup は処理後にローカルリポジトリをクリーンな状態に戻します。
	Cleanup(ctx context.Context) error
}

// GitAdapter は GitService インターフェースを実装する具体的な構造体です。
type GitAdapter struct {
	LocalPath                string
	SSHKeyPath               string
	BaseBranch               string
	InsecureSkipHostKeyCheck bool
	auth                     transport.AuthMethod
	repo                     *git.Repository
}

// Option はGitAdapterの初期化オプションを設定するための関数です。
type Option func(*GitAdapter)

// WithInsecureSkipHostKeyCheck はSSHホストキーチェックをスキップするオプションを設定します。
func WithInsecureSkipHostKeyCheck(skip bool) Option {
	return func(ga *GitAdapter) {
		ga.InsecureSkipHostKeyCheck = skip
	}
}

// WithBaseBranch はベースブランチを設定するオプションです。
func WithBaseBranch(branch string) Option {
	return func(ga *GitAdapter) {
		ga.BaseBranch = branch
	}
}

// NewGitAdapter は GitAdapter を初期化します。
func NewGitAdapter(localPath string, sshKeyPath string, opts ...Option) GitService {
	adapter := &GitAdapter{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}

	for _, opt := range opts {
		opt(adapter)
	}

	return adapter
}

// getRepository は、内部で保持しているリポジトリインスタンスを取得するヘルパー関数です。
func (ga *GitAdapter) getRepository() (*git.Repository, error) {
	if ga.repo == nil {
		// リポジトリがまだオープンされていないか、クローンされていない場合
		repo, err := git.PlainOpen(ga.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("内部リポジトリのオープンに失敗: %w", err)
		}
		ga.repo = repo
	}
	return ga.repo, nil
}

// CloneOrUpdate はリポジトリをクローンするか、既に存在する場合は更新します。
func (ga *GitAdapter) CloneOrUpdate(ctx context.Context, repositoryURL string) error {
	localPath := ga.LocalPath
	var repo *git.Repository
	var err error

	// 認証情報の取得と保持を最初に行う (NOTE: getAuthMethod は外部関数と仮定)
	auth, err := ga.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	ga.auth = auth // 認証情報を Adapter に設定
	slog.Info("go-git用の認証情報がアダプタに設定されました。")

	// --- クローン・更新ロジック ---

	_, err = os.Stat(localPath)
	if os.IsNotExist(err) {
		// 1. ローカルパスが存在しない場合はクローン
		slog.Info("リポジトリが存在しないため、クローンします。", "url", repositoryURL, "path", localPath, "branch", ga.BaseBranch)
		repo, err = git.PlainCloneContext(ctx, localPath, false, &git.CloneOptions{
			URL:           repositoryURL,
			ReferenceName: plumbing.NewBranchReferenceName(ga.BaseBranch),
			SingleBranch:  false, // 修正済み: フル履歴を取得するため
			Auth:          ga.auth,
		})
		if err != nil {
			return fmt.Errorf("リポジトリのクローンに失敗しました (URL: %s): %w", repositoryURL, err)
		}
	} else if err == nil {
		// 2. 既に存在する場合はオープン
		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return fmt.Errorf("既存リポジトリのオープンに失敗しました: %w", err)
		}
		// ⚠️ 修正適用: Pull の試行をスキップし、後続の Fetch に更新を委ねる
		slog.Info("既存リポジトリをオープンしました。Pull はスキップし、後続の Fetch に更新を委ねます。", "path", localPath)

		// Pull ロジックの代わりに、リモート情報を確認する (オプショナル)
		remote, remoteErr := repo.Remote("origin")
		if remoteErr != nil {
			slog.Warn("リモート 'origin' の情報が見つかりません。Fetch 時にエラーになる可能性があります。", "error", remoteErr)
		} else {
			slog.Debug("リモート 'origin' を確認しました。", "urls", remote.Config().URLs)
		}

	} else {
		return fmt.Errorf("ローカルパス '%s' の確認に失敗しました: %w", localPath, err)
	}

	// 内部にリポジトリインスタンスを保持
	ga.repo = repo
	return nil
}

// Fetch はリモートから最新の変更を取得します。
func (ga *GitAdapter) Fetch(ctx context.Context) error {
	repo, err := ga.getRepository()
	if err != nil {
		return err
	}

	slog.Info("リモートから最新の変更をフェッチしています...", "path", ga.LocalPath)
	if ga.auth == nil {
		slog.Warn("認証情報が設定されていません。プライベートリポジトリの場合、Fetchは失敗します。")
	}

	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err = repo.FetchContext(ctx, &git.FetchOptions{ // Contextを使用
		Auth:     ga.auth,
		RefSpecs: []config.RefSpec{refSpec},
		Progress: io.Discard,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("リモートからのフェッチに失敗しました: %w", err)
	}

	return nil
}

// GetCodeDiff は指定された2つのブランチ間の純粋な差分を、go-gitのみで取得します。
func (ga *GitAdapter) GetCodeDiff(ctx context.Context, baseBranch, featureBranch string) (string, error) {
	repo, err := ga.getRepository()
	if err != nil {
		return "", err
	}

	slog.Info("go-gitを使用して差分を計算しています。", "path", ga.LocalPath, "base_branch", baseBranch, "feature_branch", featureBranch)

	// --- 1. Feature Branch と Base Branch のフェッチ ---
	fetchRefSpecs := []config.RefSpec{
		config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", featureBranch, featureBranch)),
		config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", baseBranch, baseBranch)), // baseBranchもフェッチ
	}

	slog.Info("差分計算のために、両ブランチの最新情報をフェッチしています。")
	err = repo.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   fetchRefSpecs,
		Auth:       ga.auth,
		Progress:   io.Discard,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return "", fmt.Errorf("ブランチのフェッチに失敗: %w", err)
	}

	// --- 2. 差分計算ロジック ---

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

	baseCommit, err := repo.CommitObject(baseRef.Hash())
	if err != nil {
		return "", fmt.Errorf("ベースコミット '%s' の取得に失敗しました: %w", baseRef.Hash(), err)
	}

	featureCommit, err := repo.CommitObject(featureRef.Hash())
	if err != nil {
		return "", fmt.Errorf("フィーチャーコミット '%s' の取得に失敗しました: %w", featureRef.Hash(), err)
	}

	mergeBaseCommits, err := baseCommit.MergeBase(featureCommit)
	if err != nil {
		return "", fmt.Errorf("マージベースの検索に失敗しました: %w", err)
	}

	if len(mergeBaseCommits) == 0 {
		return "", fmt.Errorf("ブランチ '%s' と '%s' の間に共通の祖先が見つかりませんでした。3-dot diffは計算できません。", baseBranch, featureBranch)
	}

	mergeBaseCommit := mergeBaseCommits[0]

	baseTree, err := mergeBaseCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("マージベースのツリー取得に失敗しました: %w", err)
	}

	featureTree, err := featureCommit.Tree()
	if err != nil {
		return "", fmt.Errorf("フィーチャーブランチのツリー取得に失敗しました: %w", err)
	}

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
func (ga *GitAdapter) CheckRemoteBranchExists(ctx context.Context, branch string) (bool, error) {
	repo, err := ga.getRepository()
	if err != nil {
		return false, err
	}

	if branch == "" {
		return false, fmt.Errorf("リモートブランチの存在確認に失敗しました: ブランチ名が空です")
	}
	refName := plumbing.NewRemoteReferenceName("origin", branch)

	_, err = repo.Reference(refName, false)

	if err == plumbing.ErrReferenceNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("リモートブランチ '%s' の確認に失敗しました: %w", branch, err)
	}

	return true, nil
}

// Cleanup は処理後にローカルリポジトリディレクトリを完全に削除します。
func (ga *GitAdapter) Cleanup(ctx context.Context) error {
	slog.Info("クリーンアップ: ローカルリポジトリディレクトリを削除します。", "path", ga.LocalPath)

	if err := os.RemoveAll(ga.LocalPath); err != nil {
		return fmt.Errorf("ローカルリポジトリディレクトリ '%s' の削除に失敗しました: %w", ga.LocalPath, err)
	}
	slog.Info("クリーンアップ: ローカルリポジトリディレクトリを削除しました。", "path", ga.LocalPath)
	ga.repo = nil
	return nil
}

// recloneRepository は、既存リポジトリを削除し、再クローンします。
func (ga *GitAdapter) recloneRepository(ctx context.Context, repositoryURL, localPath, branch string) (*git.Repository, error) {
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
		}
		slog.Info("再クローンのため、既存のリポジトリディレクトリを削除しました。", "path", localPath)
	}

	repo, err := ga.cloneRepository(ctx, repositoryURL, localPath, branch)
	if err != nil {
		return nil, fmt.Errorf("リポジトリのクローンに失敗しました: %w", err)
	}

	return repo, nil
}

// cloneRepository は go-git.PlainClone を使用してクローン処理を実行するヘルパー関数です。
func (ga *GitAdapter) cloneRepository(ctx context.Context, repositoryURL, localPath, branch string) (*git.Repository, error) {
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

	slog.Info("Go-gitを使用してリポジトリのクローンを開始します。", "url", repositoryURL, "path", localPath)

	var auth transport.AuthMethod
	if ga.auth != nil {
		auth = ga.auth
	} else {
		var err error
		auth, err = ga.getAuthMethod(repositoryURL)
		if err != nil {
			return nil, fmt.Errorf("go-git クローン用の認証情報取得に失敗しました: %w", err)
		}
	}

	repo, err := git.PlainCloneContext(ctx, localPath, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  false, // 修正済み: フル履歴を取得するため
		Auth:          auth,
		Progress:      io.Discard,
	})
	if err != nil {
		return nil, fmt.Errorf("go-git クローンに失敗しました: %w", err)
	}
	slog.Info("Go-gitによるリポジトリのクローンに成功しました。")
	return repo, nil
}
