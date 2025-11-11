# ----------------------------------------------------------------------
# STEP 1: ビルドステージ (Goバイナリのコンパイル)
# ----------------------------------------------------------------------
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
# アプリケーションのソースコード全体をコピー (main.go を含む)
COPY . .
# 実行ファイルが ルート直下の main.go を起点としているため、ビルド対象をルート (.) に指定
# 実行ファイルは ./app/bin/reviewer に出力されます
RUN CGO_ENABLED=0 go build -o bin/reviewer .

# ----------------------------------------------------------------------
# STEP 2: gitの依存関係収集ステージ
# ----------------------------------------------------------------------
FROM debian:stable-slim AS git_deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    libcurl4-openssl-dev \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# ----------------------------------------------------------------------
# STEP 3: 実行ステージ (Distroless)
# ----------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12

# 2. git依存関係ステージから git バイナリと依存ライブラリをコピー (修正箇所)
# Gitバイナリをアプリケーションと同じディレクトリにコピー
COPY --from=git_deps /usr/bin/git /usr/local/bin/git
# 必須の共有ライブラリ（libc.so.6など）をコピー
COPY --from=git_deps /lib /lib
COPY --from=git_deps /usr/lib /usr/lib

# CA証明書をコピー
COPY --from=git_deps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# ENTRYPOINTは変更なし
ENTRYPOINT ["/usr/local/bin/reviewer"]
