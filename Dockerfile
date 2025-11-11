# ----------------------------------------------------------------------
# STEP 1: ビルドステージ (Goバイナリのコンパイル)
# ----------------------------------------------------------------------
# go-gitを使用しているため、外部のgitコマンドは不要ですが、CGO_ENABLED=0で静的リンクします。
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
# アプリケーションのソースコード全体をコピー
COPY . .
# 静的リンクを強制し、実行ファイルを /app/bin/reviewer に出力します
# go-gitを使用するため、外部gitコマンドの依存は完全に解消されています。
RUN CGO_ENABLED=0 go build -o bin/reviewer .

# ----------------------------------------------------------------------
# STEP 2: 実行ステージ (Distroless)
# ----------------------------------------------------------------------
# 静的にコンパイルされたバイナリとCA証明書のみを含む、最小限のイメージです。
FROM gcr.io/distroless/static-debian12

# 必要なCA証明書をコピー (HTTPS/SSH接続に必要)
# builderステージはdebianベースのため、ここからコピーします。
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# コンパイルされた静的バイナリをコピー
COPY --from=builder /app/bin/reviewer /usr/local/bin/reviewer

# エントリーポイント
ENTRYPOINT ["/usr/local/bin/reviewer"]
