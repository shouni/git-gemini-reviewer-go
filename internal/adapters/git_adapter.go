package adapters

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
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

// GitAdapter は GitService インターフェースを実装する具体的な構造体です。
type GitAdapter struct {
	LocalPath                string
	SSHKeyPath               string
	BaseBranch               string
	InsecureSkipHostKeyCheck bool
	auth                     transport.AuthMethod // git_auth.go で設定される認証メソッド
	// NOTE: その他の内部状態（例：前回クローンしたURL）があればここに追加します。
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
// GitService インターフェースを返します。
func NewGitAdapter(localPath string, sshKeyPath string, opts ...Option) GitService {
	adapter := &GitAdapter{
		LocalPath:  localPath,
		SSHKeyPath: sshKeyPath,
	}

	for _, opt := range opts {
		opt(adapter)
	}
	// NOTE: ここで adapter.auth の初期化 (getAuthMethodの呼び出し) はまだできません。

	return adapter
}

// CloneOrUpdate はリポジトリをクローンするか、既に存在する場合は go-git pull で更新します。
func (ga *GitAdapter) CloneOrUpdate(repositoryURL string) (*git.Repository, error) {
	localPath := ga.LocalPath
	var repo *git.Repository
	var err error

	// 認証情報の取得と保持を最初に行う
	// NOTE: getAuthMethodは internal/adapters/git_auth.go で定義されています。
	auth, err := ga.getAuthMethod(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("go-git用の認証情報取得に失敗しました: %w", err)
	}
	ga.auth = auth // 認証情報を Adapter に設定
	slog.Info("go-git用の認証情報がアダプタに設定されました。")

	// --- クローン・更新ロジック ---

	_, err = os.Stat(localPath)
	if os.IsNotExist(err) {
		// 1. ローカルパスが存在しない場合はクローン
		slog.Info("リポジトリが存在しないため、クローンします。", "url", repositoryURL, "path", localPath, "branch", ga.BaseBranch)
		repo, err = git.PlainClone(localPath, false, &git.CloneOptions{
			URL:           repositoryURL,
			ReferenceName: plumbing.NewBranchReferenceName(ga.BaseBranch),
			SingleBranch:  true,
			Depth:         1,
			Auth:          ga.auth, // 認証情報を渡す
		})
		if err != nil {
			return nil, fmt.Errorf("リポジトリのクローンに失敗しました (URL: %s): %w", repositoryURL, err)
		}
	} else if err == nil {
		// 2. 既に存在する場合はオープンして更新 (Pull)
		repo, err = git.PlainOpen(localPath)
		if err != nil {
			return nil, fmt.Errorf("既存リポジトリのオープンに失敗しました: %w", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			return nil, fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
		}

		slog.Info("既存リポジトリを更新 (Pull) します。", "path", localPath)
		err = w.Pull(&git.PullOptions{
			RemoteName: "origin",
			Auth:       ga.auth, // 認証情報を渡す
			Progress:   io.Discard,
		})

		if err != nil && err != git.NoErrAlreadyUpToDate {
			if strings.Contains(err.Error(), "pull failed, reclone required") {
				slog.Info("リカバリのための再クローンを開始します...")
				repo, err = ga.recloneRepository(repositoryURL, ga.LocalPath, ga.BaseBranch)
				if err != nil {
					return nil, err // 再クローン失敗時はエラーを返す
				}
			} else {
				return nil, fmt.Errorf("既存リポジトリの更新 (Pull) に失敗しました: %w", err) // エラーメッセージを明確化
			}
		}
	} else {
		return nil, fmt.Errorf("ローカルパス '%s' の確認に失敗しました: %w", localPath, err)
	}

	return repo, nil
}

// Fetch はリモートから最新の変更を取得します。
func (ga *GitAdapter) Fetch(repo *git.Repository) error {
	slog.Info("リモートから最新の変更をフェッチしています...", "path", ga.LocalPath)
	if ga.auth == nil {
		slog.Warn("認証情報が設定されていません。プライベートリポジトリの場合、Fetchは失敗します。")
		// NOTE: 認証情報のチェックは警告に留めます。パブリックリポジトリのFetchは成功するためです。
	}

	refSpec := config.RefSpec("+refs/heads/*:refs/remotes/origin/*")

	err := repo.Fetch(&git.FetchOptions{
		Auth:     ga.auth, // ga.auth を使用
		RefSpecs: []config.RefSpec{refSpec},
		Progress: io.Discard,
	})

	if err != nil && err != git.NoErrAlreadyUpToDate {
		// リモートが見つからない場合など、認証以外のエラーも考慮
		return fmt.Errorf("リモートからのフェッチに失敗しました: %w", err)
	}

	return nil
}

// GetCodeDiff は指定された2つのブランチ間の純粋な差分を、go-gitのみで取得します。
func (ga *GitAdapter) GetCodeDiff(repo *git.Repository, baseBranch, featureBranch string) (string, error) {
	slog.Info("go-gitを使用して差分を計算しています。", "path", ga.LocalPath, "base_branch", baseBranch, "feature_branch", featureBranch)

	// 1. ブランチ参照を解決 (リモート参照を使用)
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

	// 2. コミットオブジェクトを取得 (ハッシュから)
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
func (ga *GitAdapter) CheckRemoteBranchExists(repo *git.Repository, branch string) (bool, error) {
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
func (ga *GitAdapter) Cleanup(repo *git.Repository) error {
	slog.Info("クリーンアップ: ベースブランチへのチェックアウトを開始します。", "base_branch", ga.BaseBranch)

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("ワークツリーの取得に失敗しました: %w", err)
	}

	// ローカルの状態を破棄し、BaseBranchにチェックアウトする
	err = w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(ga.BaseBranch), // <-- NewBranchReferenceNameを使用するように修正
		Force:  true,
	})

	if err != nil {
		// エラーメッセージの改善: ブランチ参照名を明記
		return fmt.Errorf("ベースブランチ '%s' へのチェックアウトに失敗しました: %w", plumbing.NewBranchReferenceName(ga.BaseBranch).String(), err)
	}

	slog.Info("クリーンアップ: ローカルリポジトリをベースブランチにリセットしました。", "base_branch", ga.BaseBranch)
	return nil
}

// recloneRepository は、既存リポジトリを削除し、再クローンします。
func (ga *GitAdapter) recloneRepository(repositoryURL, localPath, branch string) (*git.Repository, error) {
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
		}
		slog.Info("再クローンのため、既存のリポジトリディレクトリを削除しました。", "path", localPath)
	}

	if err := ga.cloneRepository(repositoryURL, localPath, branch); err != nil {
		return nil, fmt.Errorf("リポジトリのクローンに失敗しました: %w", err)
	}

	repo, err := git.PlainOpen(localPath)
	if err != nil {
		return nil, fmt.Errorf("クローン後のリポジトリのオープンに失敗しました: %w", err)
	}
	return repo, nil
}

// cloneRepository は go-git.PlainClone を使用してクローン処理を実行するヘルパー関数です。
func (ga *GitAdapter) cloneRepository(repositoryURL, localPath, branch string) error {
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

	slog.Info("Go-gitを使用してリポジトリのクローンを開始します。", "url", repositoryURL, "path", localPath)

	auth, err := ga.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git クローン用の認証情報取得に失敗しました: %w", err)
	}

	_, err = git.PlainClone(localPath, false, &git.CloneOptions{
		URL:           repositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
		SingleBranch:  true,
		Auth:          auth,
		Progress:      io.Discard,
	})
	if err != nil {
		return fmt.Errorf("go-git クローンに失敗しました: %w", err)
	}
	slog.Info("Go-gitによるリポジトリのクローンに成功しました。")
	return nil
}
