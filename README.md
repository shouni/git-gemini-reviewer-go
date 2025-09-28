# 🤖 Git Gemini Reviewer Go

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)


## 🚀 概要 (About) - 開発チームの生産性を高めるAIパートナー

**`git-gemini-reviewer`** は、**Google Gemini の強力なAI**を活用し、**コードレビューを自動でお手伝い**するコマンドラインツールです。

このツールを導入することで、開発チームは単なる作業の効率化を超え、より**創造的で価値の高い業務**に集中できるようになります。AIは煩雑な初期チェックを担う、**チームの優秀な新しいパートナー**のような存在です。

### 🌸 導入がもたらすポジティブな変化

| メリット | チームへの影響 | 期待される効果 |
| :--- | :--- | :--- |
| **レビューの質とスピードアップ** | **「細かい見落とし」の心配が減ります。** AIがまず基本的なバグやコード規約をチェックしてくれるため、人間のレビュアーは設計やロジックといった**人間ならではの高度な判断**に集中できます。 | レビュー時間が短縮され、**新しい機能の開発に使える時間**が増えます。 |
| **チーム内の知識共有** | **ベテランも若手も、フィードバックの水準が一定になります。** 誰がレビューしても同じように質の高いフィードバックが得られるため、チーム全体の**コーディングスキル向上**を裏側からサポートします。 | チーム内の知識レベルが底上げされ、**属人性のリスク**が解消に向かいます。 |
| **Backlog連携でストレスフリー** | **「レビュー結果の転記」という地味な作業がなくなります。** AIがレビューコメントを自動でBacklogに投稿するため、開発者は**レビュー依頼からフィードバック確認までをスムーズに行えます**。 | **間接業務の負荷が大幅に軽減**され、チームの心理的なストレスが減ります。 |
| **導入のハードルの低さ** | **「大がかりな準備」は不要です。** 既存のGitやBacklog環境に、コマンドラインツールとして**静かに、素早く導入**できます。 | 新しい試みを**スモールスタート**で始められ、効果をすぐに実感できます。 |

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
    ターミナル（またはコマンドプロモンプト）を再起動し、以下のコマンドでGoが正しくインストールされ、PATHが通っているか確認します。

    ```bash
    # バージョンの確認
    go version
    # 例: go version go1.22.4 darwin/amd64

    # Goの環境変数が設定されているか確認 (オプション)
    go env GOROOT
    go env GOPATH
    ```

### 2\. プロジェクトのセットアップ

以下のコマンドで、このリポジトリをローカル環境にクローン（ダウンロード）します。

```bash
git clone git@github.com:shouni/git-gemini-reviewer-go.git
cd git-gemini-reviewer-go
```

### 3\. プロジェクトのビルド

プロジェクトのルートディレクトリで以下のコマンドを実行し、実行可能ファイルを生成します。

  * **macOS / Linux:**
    ```bash
    go build -o git-gemini-reviewer-go
    ```
  * **Windows:**
    ```bash
    go build -o git-gemini-reviewer-go.exe
    ```
    実行ファイルがカレントディレクトリに生成されます。

-----

### 4\. 環境変数の設定 (必須)

Gemini API を利用するために、API キーを環境変数に設定する必要があります。Backlog 連携を使用する場合は、Backlog の情報も設定してください。

#### macOS / Linux (bash/zsh)

```bash
# Gemini API キー (必須)
export GEMINI_API_KEY="YOUR_GEMINI_API_KEY"

# Backlog 連携を使用する場合 (`backlog` コマンド利用時のみ)
export BACKLOG_API_KEY="YOUR_BACKLOG_API_KEY"
export BACKLOG_SPACE_URL="https://your-space.backlog.jp"
```

#### Windows (PowerShell)

```powershell
# Gemini API キー (必須)
$env:GEMINI_API_KEY="YOUR_GEMINI_API_KEY"

# Backlog 連携を使用する場合 (`backlog` コマンド利用時のみ)
$env:BACKLOG_API_KEY="YOUR_BACKLOG_API_KEY"
$env:BACKLOG_SPACE_URL="https://your-space.backlog.jp"
```

> **Note:** 環境変数を恒久的に設定するには、シェルの設定ファイル (`.zshrc`, `.bash_profile` など) や、Windowsの「環境変数」設定画面で編集してください。

-----

### 5\. プロンプトファイルの準備 (必須)

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

**`%s`** は、取得したコード差分が挿入されるプレースホルダーです。**必ず含めてください。**

-----

## 🚀 使い方 (Usage)

生成された実行ファイルを使用し、サブコマンドとフラグを指定して実行します。

### 1\. 汎用レビューモード (`generic`)

レビュー結果をターミナルに直接出力します。

#### 実行コマンド例

  * **macOS / Linux:**
    ```bash
    ./git-gemini-reviewer-go generic \
      --git-clone-url "git@github.com:your-org/your-repo.git" \
      --base-branch "main" \
      --feature-branch "feature/new-feature-branch"
    ```
  * **Windows (PowerShell):**
    ```powershell
    .\git-gemini-reviewer-go.exe generic `
      --git-clone-url "git@github.com:your-org/your-repo.git" `
      --base-branch "main" `
      --feature-branch "feature/new-feature-branch"
    ```

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--git-clone-url` | レビュー対象のGitリポジトリURL（SSH形式推奨） | ✅ | なし |
| `--base-branch` | 差分比較の基準ブランチ | ✅ | なし |
| `--feature-branch` | レビュー対象のフィーチャーブランチ | ✅ | なし |
| `--ssh-key-path` | SSH認証用の秘密鍵パス | ❌ | `~/.ssh/id_ed25519` |
| `--prompt-file` | プロンプトファイルのパス | ❌ | `review_prompt.md` |
| `--local-path` | リポジトリのクローン先 | ❌ | OSの一時ディレクトリ |

-----

### 2\. Backlog 投稿モード (`backlog`)

レビュー結果を Backlog の課題にコメントとして投稿します。

#### 実行コマンド例

  * **macOS / Linux:**
    ```bash
    ./git-gemini-reviewer-go backlog \
      --git-clone-url "git@example.backlog.jp:PROJECT/repo-name.git" \
      --base-branch "develop" \
      --feature-branch "bugfix/issue-456" \
      --issue-id "PROJECT-123"
    ```
  * **Windows (PowerShell):**
    ```powershell
    .\git-gemini-reviewer-go.exe backlog `
      --git-clone-url "git@example.backlog.jp:PROJECT/repo-name.git" `
      --base-branch "develop" `
      --feature-branch "bugfix/issue-456" `
      --issue-id "PROJECT-123"
    ```

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--issue-id` | コメントを投稿するBacklog課題ID（例: PROJECT-123） | ✅ | なし |
| `--no-post` | Backlogへのコメント投稿をスキップし、結果を標準出力する | ❌ | `false` |
| **その他のフラグ** | **`generic` モードと同じ** | | |

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
