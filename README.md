このCLIツールのための包括的なREADMEを作成します。技術スタック、Goのインストール手順、そしてコマンドラインでの具体的な使用方法までを網羅します。

-----

# 🤖 Git Gemini Reviewer Go

**Gemini AI を利用して Git の差分をレビューし、その結果を標準出力または Backlog にコメント投稿するコマンドラインツールです。**

このツールは、GitHub や GitLab など、SSH アクセスが可能な任意の Git リポジトリで動作し、コードレビューのプロセスを自動化・高速化します。

-----

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| :--- | :--- | :--- |
| **言語** | **Go (Golang)** | ツールの開発言語。クロスプラットフォームでの高速な実行を実現します。 |
| **CLI フレームワーク** | **Cobra** | コマンドライン引数（フラグ）の解析とサブコマンド構造 (`generic`, `backlog`) の構築に使用します。 |
| **Git 操作** | **go-git** | リポジトリのクローン、フェッチ、およびブランチ間の差分 (`git diff`) の取得に使用します。SSH認証に対応しています。 |
| **AI モデル** | **Google Gemini API** | 取得したコード差分を分析し、レビューコメントを生成するために使用します。 |
| **Backlog 連携** | **標準 `net/http`** | Backlog API (REST API) を使用して、生成されたレビュー結果を課題にコメントとして投稿します。 |

-----

## 🛠️ 事前準備と環境設定

### 1\. Go のインストール

このツールを実行・ビルドするには、Go言語の環境が必要です。

1.  **インストーラーのダウンロード:**
    [Go の公式サイト](https://go.dev/dl/)から、お使いのOSに合ったインストーラーをダウンロードし、実行してください。

2.  **インストールの確認:**
    ターミナルを再起動し、以下のコマンドでGoが正しくインストールされたか確認します。

    ```bash
    go version
    # 例: go version go1.22.0 darwin/amd64
    ```

### 2\. プロジェクトのビルド

プロジェクトのルートディレクトリで以下のコマンドを実行し、実行可能ファイルを生成します。

```bash
go build
# 実行可能ファイルがカレントディレクトリに生成されます (例: ./git-gemini-reviewer-go)
```

### 3\. 環境変数の設定 (必須)

Gemini API を利用するために、APIキーを環境変数に設定する必要があります。Backlog 連携を使用する場合は、Backlog の情報も設定してください。

```bash
# Gemini API キー (必須)
export GEMINI_API_KEY="YOUR_GEMINI_API_KEY"

# Backlog 連携を使用する場合 (backlog コマンド利用時のみ)
export BACKLOG_API_KEY="YOUR_BACKLOG_API_KEY"
export BACKLOG_SPACE_URL="https://your-space.backlog.jp"
```

### 4\. プロンプトファイルの準備 (必須)

プロジェクトのルートディレクトリに、Gemini にレビューを依頼する際の指示を記述した **`review_prompt.md`** ファイルを作成してください。

**`review_prompt.md` の内容例:**

```markdown
あなたは経験豊富なシニアソフトウェアエンジニアです。以下のGit差分（Diff）をレビューしてください。
コード品質、セキュリティ、パフォーマンス、可読性、ベストプラクティスからの逸脱について、簡潔かつ建設的なレビューを日本語で行ってください。
レビュー結果はMarkdown形式で、必ず「## レビュー結果」という見出しから始めてください。

---
Git Diff:
%s 
---
```

**`%s`** は、取得したコード差分が挿入されるプレースホルダーです。必ず含めてください。

-----

## 🚀 使い方 (Usage)

生成された実行ファイル **`./git-gemini-reviewer-go`** を使用し、サブコマンドとフラグを指定して実行します。

### 1\. 汎用レビューモード (`generic`)

レビュー結果をターミナルに直接出力します。

#### 実行コマンド例

```bash
./git-gemini-reviewer-go generic \
  --git-clone-url "git@github.com:your-org/your-repo.git" \
  --base-branch "main" \
  --feature-branch "feature/new-feature-branch"
```

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--git-clone-url` | レビュー対象のGitリポジトリURL（SSH形式推奨） | ✅ | なし |
| `--base-branch` | 差分比較の基準ブランチ | ✅ | なし |
| `--feature-branch` | レビュー対象のフィーチャーブランチ | ✅ | なし |
| `--ssh-key-path` | SSH認証用の秘密鍵パス | ❌ | `~/.ssh/id_rsa` |
| `--prompt-file` | プロンプトファイルのパス | ❌ | `review_prompt.md` |
| `--local-path` | リポジトリのクローン先 | ❌ | OSの一時ディレクトリ |

-----

### 2\. Backlog 投稿モード (`backlog`)

レビュー結果を Backlog の課題にコメントとして投稿します。

#### 実行コマンド例

```bash
./git-gemini-reviewer-go backlog \
  --git-clone-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "develop" \
  --feature-branch "bugfix/issue-456" \
  --issue-id "PROJECT-123" 
```

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--issue-id` | コメントを投稿するBacklog課題ID（例: PROJECT-123） | ✅ | なし |
| `--no-post` | Backlogへのコメント投稿をスキップし、結果を標準出力する | ❌ | `false` |
| **その他のフラグ** | **`generic` モードと同じ** | | |