package adapters

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

// NOTE: このファイルには GitAdapter 構造体の定義と、
// CloneOrUpdate メソッドなどから呼び出される getAuthMethod が含まれます。

// --- ヘルパー関数 ---

// expandTilde はクロスプラットフォームなチルダ展開をサポートする
func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") {
		return path, nil
	}
	// os/userパッケージの利用は、クロスコンパイル環境によっては問題になる可能性がありますが、
	// 通常のアプリケーションでは標準的なアプローチです。
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("現在のユーザーのホームディレクトリの取得に失敗しました: %w", err)
	}
	return filepath.Join(currentUser.HomeDir, path[2:]), nil
}

// --- 認証メソッドの実装 (GitAdapterに修正) ---

// getAuthMethod は go-git が使用する認証方法を返します。
// GitAdapter の設定に基づいて SSH 認証を構築します。
func (ga *GitAdapter) getAuthMethod(repoURL string) (transport.AuthMethod, error) { // <-- レシーバーを *GitAdapter に修正
	if strings.HasPrefix(repoURL, "git@") || strings.HasPrefix(repoURL, "ssh://") {

		// 1. リポジトリURLの解析とユーザー名の決定 (go-gitは 'git' ユーザーを想定することが多い)
		u, err := url.Parse(repoURL)
		if err != nil {
			// git@github.com:user/repo.git のようなSSH短縮形の場合、url.Parseは失敗します。
			// その場合は、デフォルトの"git"ユーザーを使用します。
			if !strings.HasPrefix(repoURL, "git@") {
				return nil, fmt.Errorf("リポジトリURLのパースに失敗しました: %w", err)
			}
		}

		username := "git"
		if u != nil && u.User != nil {
			username = u.User.Username()
		}

		// 2. SSHキーパスの展開とファイルの読み込み
		sshKeyPath, err := expandTilde(ga.SSHKeyPath) // <-- ga.SSHKeyPath を使用
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

		// 3. PublicKeys 認証メソッドの生成
		// パスフレーズが空 ("") の SSH キーを想定
		auth, err := ssh.NewPublicKeys(username, sshKey, "")
		if err != nil {
			return nil, fmt.Errorf("SSH認証キーのロードに失敗しました: %w", err)
		}

		// 4. HostKeyCallback の設定
		// InsecureSkipHostKeyCheck の設定を適用
		if ga.InsecureSkipHostKeyCheck { // <-- ga.InsecureSkipHostKeyCheck を使用
			auth.HostKeyCallback = cryptossh.InsecureIgnoreHostKey()
		} else {
			// HostKeyCallbackがnilの場合、go-gitは標準的なknown_hostsのチェックを実行します。
			auth.HostKeyCallback = nil
		}

		return auth, nil
	}

	// SSH ではないリポジトリURLの場合（例：https://）
	// go-git は通常、HTTP/HTTPSリポジトリに対しては認証なし（nil）でアクセスを試みます。
	// 必要であれば、HTTP Basic Auth などのロジックを追加できます。
	return nil, nil
}
