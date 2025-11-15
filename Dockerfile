# ----------------------------------------------------------------------
# STEP 1: ビルドステージ (Goバイナリのコンパイル)
# ----------------------------------------------------------------------
FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o bin/gemini_reviewer .

RUN mkdir -p /root/.ssh
COPY known_hosts /root/.ssh/known_hosts
RUN chmod 600 /root/.ssh/known_hosts

# ----------------------------------------------------------------------
# STEP 2: 実行ステージ (実行専用の超軽量・セキュアなイメージ)
# ----------------------------------------------------------------------
FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.source=https://github.com/shouni/git-gemini-reviewer-go \
      org.opencontainers.image.description="A Go application for reviewing code diffs using Google Gemini." \
      org.opencontainers.image.url="https://github.com/shouni/git-gemini-reviewer-go"

# ビルドステージの相対パス (/app/bin/gemini_reviewer) からコピー
COPY --from=builder /app/bin/gemini_reviewer /app/gemini_reviewer

RUN mkdir -p /root/.ssh
COPY --from=builder /root/.ssh/known_hosts /root/.ssh/known_hosts
# エントリポイントを定義
ENTRYPOINT ["/app/gemini_reviewer"]
