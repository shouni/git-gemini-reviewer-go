package gitclient

import (
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

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
func (c *Client) getAuthMethod(repoURL string) (transport.AuthMethod, error) {
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
