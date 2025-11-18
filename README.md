# 🤖 Git Gemini Reviewer Go

[![Language](https://img.shields.io/badge/Language-Go-blue)](https://golang.org/)
[![Go Version](https://img.shields.io/github/go-mod/go-version/shouni/git-gemini-reviewer-go)](https://golang.org/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/shouni/git-gemini-reviewer-go)](https://github.com/shouni/git-gemini-reviewer-go/tags)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## 🚀 概要 (About) - 開発チームの生産性を高めるAIパートナー

**Git Gemini Reviewer Go** は、AIコードレビューの**コア機能**を外部の [`gemini-reviewer-core`](https://github.com/shouni/gemini-reviewer-core) モジュールに依存し、それをCLIとして公開するための**ラッパーアプリケーション**です。

本ツールは、ユーザーの入力（CLIフラグ）を解析し、外部コアライブラリを組み合わせて**レビューパイプライン全体を実行**し、結果を様々なサービス（Slack, GCS, Backlog, 標準出力）に投稿する**インターフェースの責務**を担います。AIは煩雑な初期チェックを担う、**チームの優秀な新しいパートナー**のような存在です。

### 🌸 導入がもたらすポジティブな変化

| メリット | チームへの影響 | 期待される効果 |
| :--- | :--- | :--- |
| **レビューの質とスピードアップ** | **「細かい見落とし」の心配が減ります。** AIがまず基本的なバグやコード規約をチェックしてくれるため、人間のレビュアーは設計やロジックといった**人間ならではの高度な判断**に集中できます。 | レビュー時間が短縮され、**新しい機能の開発に使える時間**が増えます。 |
| **連携の多様化と柔軟性** | **「フィードバックの場所」を自由に選べます。** **Backlog**、**Slack**、**GCS** (Google Cloud Storage)、標準出力など、チームのワークフローに合わせて最適な場所に結果を届けられます。 | **間接業務の負荷が大幅に軽減**され、チームの心理的なストレスが減ります。 |
| **チーム内の知識共有** | **ベテランも若手も、フィードバックの水準が一定になります。** 誰がレビューしても同じように質の高いフィードバックが得られるため、チーム全体の**コーディングスキル向上**を裏側からサポートします。 | チーム内の知識レベルが底上げされ、**属人性のリスク**が解消に向かいます。 |

このツールは、GitHub や GitLab など、**SSH アクセスが可能な任意の Git リポジトリ**で動作します。

-----

## ✨ 技術スタック (Technology Stack)

| 要素 | 技術 / ライブラリ | 役割 |
| :--- | :--- | :--- |
| **言語** | **Go (Golang)** | ツールの開発言語。クロスプラットフォームでの高速な実行を実現します。 |
| **CLI フレームワーク** | **Cobra** | コマンドライン引数（フラグ）の解析とサブコマンド構造 (`generic`, `backlog`, `slack`, `gcs`) の構築に使用します。 |
| **コアロジック** | **`github.com/shouni/gemini-reviewer-core`** | **Git操作**、**AI通信**、**HTML変換**といった中核のレビュー機能を担う外部ライブラリです。 |
| **ロギング** | **log/slog** | 構造化されたログ (`key=value`) に完全移行。詳細なデバッグ情報が必要な際に、ログレベルを上げて柔軟に対応できます。 |
| **堅牢性** | **cenkalti/backoff** (内部移植) | **AI API通信**、**Slack**、**Backlog**への投稿処理に**リトライ機構**を実装。一時的なネットワーク障害やAPIのレート制限からの自動回復を実現します。 |
| 連携サービス | Slack Go ライブラリ (slack-go/slack) / 標準 net/http | Slack Block Kit を使用したリッチなメッセージングと、Backlog API への投稿に使用します。 |

-----

## 🛠️ 事前準備と環境設定

### 1\. Go のインストール

本ツールは Go言語で開発されています。Goが未インストールの場合は、[公式ドキュメント](https://go.dev/doc/install) を参照し、環境に合わせたインストールを行ってください。

### 2\. プロジェクトのセットアップとビルド

```bash
# リポジトリをクローン
git clone git@github.com:shouni/git-gemini-reviewer-go.git

# 実行ファイルを bin/ ディレクトリに生成
go build -o bin/gemini_reviewer
```

実行ファイルは、プロジェクトルートの `./bin/gemini_reviewer` に生成されます。

-----

### 3\. 環境変数の設定 (必須)

Gemini API を利用するために、API キーを環境変数に設定する必要があります。また、連携サービスを使用する場合は、対応する環境変数を設定します。

```bash
# Gemini API キー (必須)
export GEMINI_API_KEY="YOUR_GEMINI_API_KEY"

# Backlog 連携を使用する場合 (`backlog` コマンド利用時のみ)
export BACKLOG_API_KEY="YOUR_BACKLOG_API_KEY"
export BACKLOG_SPACE_URL="https://your-space.backlog.jp"

# Slack 連携を使用する場合 (`slack` コマンド利用時のみ)
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/..."
```

-----

### 4\. モデルパラメータとプロンプト設定について (重要) 🆕

本ツールが使用する以下のコア設定は、依存先の[`gemini-reviewer-core`](https://github.com/shouni/gemini-reviewer-core) モジュール内で定義・管理されています。

* **温度 (Temperature):** `0.2` に設定されています。
    * この低い温度設定は、応答の安定性を優先し、一貫性のあるコードレビュー結果を生成するために、コアライブラリ側で適用されています。
* **プロンプト設定:** プロンプトテンプレートファイル (`.md`) は、**コアライブラリのリポジトリ**に配置されており、本ツールでは**変更できません**。内容を確認・変更したい場合は、**[`gemini-reviewer-core` のリポジトリ](https://github.com/shouni/gemini-reviewer-core)** を参照してください。

-----

## 🤖 AIコードレビューの種類 (`--mode` オプション)

本ツールは、レビューの目的に応じて AI に与える指示（**プロンプト**）を切り替えることができます。これは共通フラグの **`-m`, `--mode`** で指定します。

| モード (`-m`) | プロンプトファイル (参照先) | 目的とレビュー観点 |
| :--- | :--- | :--- |
| **`detail`** | **gemini-reviewer-core/prompts/prompt\_detail.md** | **コード品質と保守性の向上**を目的とした詳細なレビュー。可読性、重複、命名規則、一般的なベストプラクティスからの逸脱など、広範囲な技術的側面に焦点を当てます。 |
| **`release`** | **gemini-reviewer-core/prompts/prompt\_release.md** | **本番リリース可否の判定**を目的としたクリティカルなレビュー。致命的なバグ、セキュリティ脆弱性、サーバーダウンにつながる重大なパフォーマンス問題など、リリースをブロックする問題に限定して指摘します。 |

-----

## 🚀 使い方 (Usage) と実行例

このツールは、**リモートリポジトリのブランチ間比較**に特化しており、**サブコマンド**を使用します。

### 🛠 共通フラグ (Persistent Flags)

すべてのサブコマンド (`generic`, `backlog`, `slack`, `gcs`) で使用可能なフラグです。

| フラグ | ショートカット | 説明 | デフォルト値 | 必須 |
| :--- | :--- | :--- | :--- | :--- |
| `--mode` | **`-m`** | レビューモードを指定: `'release'` (リリース判定) または `'detail'` (詳細レビュー) | `detail` | ❌ |
| `--repo-url` | **`-u`** | レビュー対象の Git リポジトリの **SSH URL** | **なし** | ✅ |
| `--base-branch` | **`-b`** | 差分比較の基準ブランチ | `main` | ❌ |
| `--feature-branch` | **`-f`** | レビュー対象のフィーチャーブランチ | **なし** | ✅ |
| `--local-path` | **`-l`** | リポジトリをクローンするローカルパス | 一時ディレクトリ | ❌ |
| `--gemini` | **`-g`** | 使用する Gemini モデル名 (例: `gemini-2.5-flash`) | `gemini-2.5-flash` | ❌ |
| `--ssh-key-path` | **`-k`** | Git 認証用の SSH 秘密鍵のパス。**チルダ (`~`) 展開をサポート**しています。**CI/CD環境ではシークレットマウント先の絶対パス**を指定してください。 | `~/.ssh/id_rsa` | ❌ |
| `--skip-host-key-check` | なし | SSHホストキーチェックをスキップする（**🚨非推奨/危険な設定**）。**`known_hosts`を使用しない**場合に設定します。 | `false` | ❌ |

-----

### 1\. 標準出力モード (`generic`)

リモートリポジトリのブランチ差分を取得し、レビュー結果を**標準出力**に出力します。

#### 実行コマンド例

```bash
# main と develop の差分をリリース判定モードで実行
./bin/gemini_reviewer generic \
  -m "release" \
  --repo-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "main" \
  --feature-branch "develop"
```

-----

### 2\. GCS 保存モード (`gcs`) 🆕

リモートリポジトリのブランチ比較を行い、その結果を **Google Cloud Storage (GCS)** の指定された URI に、**AIが出力したMarkdownを専用ライブラリ（go-text-format）で変換したスタイル付き HTML** として保存します。このモードは、レビュー結果のアーカイブや、CI/CDパイプラインでのレポート生成を目的としています。

#### 実行コマンド例

```bash
# feature/gcs-save の差分をレビューし、GCSにHTML結果を保存
./bin/gemini_reviewer gcs \
  -m "detail" \
  --repo-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "main" \
  --feature-branch "feature/gcs-save" \
  --gcs-uri "gs://review-archive-bucket/reviews/2025/latest_review.html" 
```

#### 固有フラグ (GCS連携)

| フラグ | ショートカット | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- | :--- |
| `--gcs-uri` | **`-s`** | 書き込み先 GCS URI (例: `gs://bucket/path/to/result.html`) | ❌ | `gs://git-gemini-reviewer-go/review/result.html` |
| `--content-type` | **`-t`** | GCSに保存するファイルのMIMEタイプ | ❌ | **`text/html; charset=utf-8`** |

-----

### 3\. Backlog 投稿モード (`backlog`) 🌟

リモートリポジトリのブランチ比較を行い、その結果を Backlog の指定された課題に**コメントとして投稿**します。投稿失敗時には**リトライ機構**が働きます。

#### 実行コマンド例

```bash
# bugfix/issue-456 の差分をレビューし、PROJECT-123 に投稿
./bin/gemini_reviewer backlog \
  --repo-url "git@example.backlog.jp:PROJECT/repo-name.git" \
  --base-branch "main" \
  --feature-branch "bugfix/issue-456" \
  -i "PROJECT-123" 
```

#### 固有フラグ (Backlog連携)

| フラグ | ショートカット | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- | :--- |
| `--issue-id` | **`-i`** | コメントを投稿する Backlog 課題 ID (例: PROJECT-123) | **投稿時のみ✅** | なし |
| `--no-post` | なし | Backlog への投稿をスキップし、結果を標準出力する | ❌ | `false` |

-----

### 4\. Slack 投稿モード (`slack`) 🌟 (Block Kit 対応済み)

リモートリポジトリのブランチ比較を行い、その結果を **Slack の Webhook URL** を通じてメッセージとして投稿します。投稿失敗時には**リトライ機構**が働き、リポジトリ情報を元にした**自動識別子抽出**も行われます。

**【重要な変更点】** Slack Go ライブラリ(slack-go/slack)を使用した **Slack Block Kit 形式**で投稿されるようになりました。これにより、通知が構造化され、視認性が大幅に向上しました。さらに、通知には**レビュー対象のブランチ名とリポジトリ名**が含められます。

#### 実行コマンド例

```bash
# feature/slack-notify の差分を詳細レビューモードで実行し、Slackに投稿
./bin/gemini_reviewer slack \
  -m "detail" \
  --repo-url "ssh://github.com/owner/repo-name.git" \
  --base-branch "main" \
  --feature-branch "feature/slack-notify" 
```

#### 固有フラグ (Slack連携)

| フラグ | 説明 | 必須 | デフォルト値 |
| :--- | :--- | :--- | :--- |
| `--no-post` | Slack への投稿をスキップし、結果を標準出力する | ❌ | `false` |

-----

### 📜 ライセンス (License)

このプロジェクトは [MIT License](https://opensource.org/licenses/MIT) の下で公開されています。
