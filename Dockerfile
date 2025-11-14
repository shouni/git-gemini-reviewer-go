# ----------------------------------------------------------------------
# STEP 1: ビルドステージ (Goバイナリのコンパイル)
# ----------------------------------------------------------------------
FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
# アプリケーションのソースコード全体をコピー (main.go を含む)
COPY . .
# 実行ファイルが ルート直下の main.go を起点としているため、ビルド対象をルート (.) に指定
# 実行ファイルは ./app/bin/gemini_reviewer に出力されます
RUN CGO_ENABLED=0 go build -o bin/gemini_reviewer .

# ----------------------------------------------------------------------
# STEP 2: 実行ステージ (実行専用の超軽量・セキュアなイメージ)
# ----------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.source=https://github.com/shouni/git-gemini-reviewer-go \
      org.opencontainers.image.description="A Go application for reviewing code diffs using Google Gemini." \
      org.opencontainers.image.url="https://github.com/shouni/git-gemini-reviewer-go"

# ビルドステージの相対パス (/app/bin/gemini_reviewer) からコピー
COPY --from=builder /app/bin/gemini_reviewer /app/gemini_reviewer

# エントリポイントを定義
ENTRYPOINT ["/app/gemini_reviewer"]
