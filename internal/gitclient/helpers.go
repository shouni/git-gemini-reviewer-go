package gitclient

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// cloneRepository は go-git.PlainClone を使用してクローン処理を実行するヘルパー関数です。
func (c *Client) cloneRepository(repositoryURL, localPath, branch string) error {
	parentDir := filepath.Dir(localPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("親ディレクトリの作成に失敗しました: %w", err)
		}
	}

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
	slog.Info("Go-gitによるリポジトリのクローンに成功しました。")
	return nil
}

// recloneRepository は、既存リポジトリを削除し、再クローンします。
func (c *Client) recloneRepository(repositoryURL, localPath, branch string) (*git.Repository, error) {
	if _, err := os.Stat(localPath); err == nil {
		if err := os.RemoveAll(localPath); err != nil {
			return nil, fmt.Errorf("既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", localPath, err)
		}
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

// updateExistingRepository は、既存リポジトリをプルで更新し、失敗した場合は再クローンが必要なエラーを返します。
func (c *Client) updateExistingRepository(repo *git.Repository, repositoryURL string) error {
	authForPull, err := c.getAuthMethod(repositoryURL)
	if err != nil {
		return fmt.Errorf("go-git pull用の認証情報取得に失敗しました: %w", err)
	}

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
		slog.Info("go-git pullによるリポジトリの更新に成功しました。")
		return nil
	}

	slog.Info("警告: go-git pullに失敗しました。リカバリのために再クローンを試行します。", "error", pullErr)
	if err := os.RemoveAll(c.LocalPath); err != nil {
		return fmt.Errorf("pull失敗後の既存リポジトリディレクトリ (%s) の削除に失敗しました: %w", c.LocalPath, err)
	}

	return fmt.Errorf("pull failed, reclone required: %w", pullErr)
}

// repoNeedsReclone はリポジトリを再クローンする必要があるかをチェックするヘルパー関数
func (c *Client) repoNeedsReclone(repositoryURL, localPath string) bool {
	gitDir := filepath.Join(localPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		slog.Info(".gitディレクトリが見つかりません。クローンが必要です。", "path", localPath)
		return true
	}
	repo, err := git.PlainOpen(localPath)
	if err != nil {
		slog.Warn("既存のリポジトリを開けませんでした。再クローンを試行します。", "path", localPath, "error", err)
		return true
	}
	remote, err := repo.Remote("origin")
	if err != nil {
		slog.Warn("既存のリポジトリにリモート 'origin' が見つかりません。再クローンを試行します。", "path", localPath, "error", err)
		return true
	}
	remoteURLs := remote.Config().URLs
	if len(remoteURLs) == 0 || remoteURLs[0] != repositoryURL {
		slog.Warn("既存リポジトリのリモートURLが要求されたURLと一致しません。再クローンを試行します。", "existing_urls", remoteURLs, "requested_url", repositoryURL)
		return true
	}
	return false
}

// getTwoDotDiff は 2-dot diff (A..B) を計算するヘルパー
// GetCodeDiff からは使用されていませんが、将来的なロジックのために残しました
func (c *Client) getTwoDotDiff(baseCommit, featureCommit *object.Commit) (string, error) {
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
