# 🤖 Git Gemini Reviewer Go

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/git-gemini-reviewer-go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - 開発チームの生産性を高めるAIパートナー

**`Git Gemini Reviewer Go`** は、**Google Gemini の強力なAI**を活用し、**コードレビューを自動でお手伝い**するコマンドラインツールです。

このツールを導入することで、開発チームは単なる作業の効率化を超え、より**創造的で価値の高い業務**に集中できるようになります。AIは煩雑な初期チェックを担う、**チームの優秀な新しいパートナー**のような存在です。

### 🌸 導入がもたらすポジティブな変化

| メリット | チームへの影響 | 期待される効果 |
| :--- | :--- | :--- |
| **レビューの質とスピードアップ** | **「細かい見落とし」の心配が減ります。** AIがまず基本的なバグやコード規約をチェックしてくれるため、人間のレビュアーは設計やロジックといった**人間ならではの高度な判断**に集中できます。 | レビュー時間が短縮され、**新しい機能の開発に使える時間**が増えます。 |
| **チーム内の知識共有** | **ベテランも若手も、フィードバックの水準が一定になります。** 誰がレビューしても同じように質の高いフィードバックが得られるため、チーム全体の**コーディングスキル向上**を裏側からサポートします。 | チーム内の知識レベルが底上げされ、**属人性のリスク**が解消に向かいます。 |
| **Backlog連携でストレスフリー** | **「レビュー結果の転記」という地味な作業がなくなります。** AIがレビューコメントを自動でBacklogに投稿するため、開発者は**レビュー依頼からフィードバック確認までをスムーズに行えます**。 | **間接業務の負荷が大幅に軽減**され、チームの心理的なストレスが減ります。 |
| **導入のハードルの低さ** | **「大がかりな準備」は不要です。** 既存のGitやBacklog環境に、コマンドラインツールとして**静かに、素早く導入**できます。 | 新しい試みを**スモールスタート**で始められ、効果をすぐに実感できます。 |

このツールは、GitHub や GitLab など、**SSH アクセスが可能な任意の Git リポジトリ**で動作します。

-----

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| :--- | :--- | :--- |
| **言語** | **Go (Golang)** | ツールの開発言語。クロスプラットフォームでの高速な実行を実現します。 |
| **CLI フレームワーク** | **Cobra** | コマンドライン引数（フラグ）の解析とサブコマンド構造 (`generic`, `backlog`) の構築に使用します。 |
| **Git 操作** | **go-git / os/exec (外部Git)** | **リモートリポジトリのブランチ間差分**の取得、クローン、フェッチに使用します。SSH認証に対応しています。 |
| **AI モデル** | **Google Gemini API** | 取得したコード差分を分析し、レビューコメントを生成するために使用します。 |
| **Backlog 連携** | **標準 `net/http`** | Backlog API (REST API) を使用して、生成されたレビュー結果を課題にコメントとして投稿します。 |

-----

## 🛠️ 事前準備と環境設定

### 1\. Go のインストール

（*省略：Go言語のインストール手順は変更なし*）

### 2\. プロジェクトのセットアップとビルド

```bash
# リポジトリをクローン
git clone git@github.com:shouni/git-gemini-reviewer-go.git
cd git-gemini-reviewer-go

# 実行ファイルを bin/ ディレクトリに生成
go build -o bin/gemini_reviewer
```

実行ファイルは、プロジェクトルートの `./bin/gemini_reviewer` に生成されます。

-----

### 3\. 環境変数の設定 (必須)

Gemini API を利用するために、API キーを環境変数に設定する必要があります。

#### macOS / Linux (bash/zsh)

```bash
# Gemini API キー (必須)
export GEMINI_API_KEY="YOUR_GEMINI_API_KEY"

# Backlog 連携を使用する場合 (`backlog` コマンド利用時のみ)
export BACKLOG_API_KEY="YOUR_BACKLOG_API_KEY"
export BACKLOG_SPACE_URL="https://your-space.backlog.jp"
```

> **Note:** 環境変数を恒久的に設定するには、シェルの設定ファイル (`.zshrc`, `.bash_profile` など) で編集してください。

-----

### 4\. プロンプトファイルの準備

現在、プロンプトはGoコード内に埋め込まれているため、外部ファイル (`review_prompt.md` 等) の準備は**不要**です。プロンプトの内容を変更したい場合は、Goコード内のファイルを直接修正してください。

-----

## 🚀 使い方 (Usage) と実行例

このツールは、**ローカルリポジトリのクイックレビュー**と、**リモートリポジトリのブランチ間レビュー**の2つの主要なユースケースに対応しています。

### 🛠 共通フラグ (Persistent Flags)

すべてのコマンド (`generic`, `backlog`, およびルートコマンド) で使用可能なフラグです。

| フラグ | 説明 | デフォルト値 |
| :--- | :--- | :--- |
| `-m`, `--mode` | レビューモードを指定します: `'release'` (リリース判定) または `'detail'` (詳細レビュー) | `detail` |
| `--model` | 使用する Gemini モデル名 (例: `gemini-2.5-flash`) | `gemini-2.5-flash` |

-----

### 1\. ローカルの Git 差分をレビュー (Root Command: `bin/gemini_reviewer`)

**現在作業中のローカルリポジトリ**の直前のコミット (`HEAD^`) と現在のコミット (`HEAD`) の差分をレビューします。リモート連携のフラグは無視されます。

#### 実行コマンド例

```bash
# 直前のコミットと現在のHEADの差分を詳細レビューモードで実行
./bin/gemini_reviewer

# リリース判定レビューモードで実行
./bin/gemini_reviewer --mode release
```

-----

### 2\. リモートリポジトリの比較をレビュー (Subcommand: `generic`)

リモートリポジトリの任意の2つのブランチ間の差分を取得し、レビュー結果を**標準出力**に出力します。

#### 実行コマンド例

```bash
# main と develop の差分を詳細レビューモードで実行
./bin/gemini_reviewer generic \
  --git-clone-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "main" \
  --feature-branch "develop"
```

#### 固有フラグ (リモートレビューに必須)

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--git-clone-url` | レビュー対象の Git リポジトリの **SSH URL** | **✅** | なし |
| `--feature-branch` | レビュー対象のフィーチャーブランチ | **✅** | なし |
| `--base-branch` | 差分比較の基準ブランチ | ❌ | `main` |
| `--ssh-key-path` | Git 認証用の SSH 秘密鍵のパス | ❌ | `~/.ssh/id_rsa` |
| `--local-path` | リポジトリのクローン先ローカルパス | ❌ | OSの一時ディレクトリ |
| `--skip-host-key-check` | SSHホストキーチェックをスキップする (非推奨) | ❌ | `false` |

-----

### 3\. Backlog 投稿モード (`backlog`)

リモートリポジトリのブランチ比較を行い、その結果を Backlog の指定された課題に**コメントとして投稿**します。

#### 実行コマンド例

```bash
# bugfix/issue-456 の差分をレビューし、PROJECT-123 に投稿
./bin/gemini_reviewer backlog \
  --git-clone-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "main" \
  --feature-branch "bugfix/issue-456" \
  --issue-id "PROJECT-123" 
```

#### 固有フラグ (Backlog連携)

Git連携フラグ (`--git-clone-url` など) は `generic` モードと**共通**です。

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--issue-id` | コメントを投稿する Backlog 課題 ID (例: PROJECT-123) | **投稿時のみ✅** | なし |
| `--no-post` | Backlog への投稿をスキップし、結果を標準出力する | ❌ | `false` |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。