package main // ⬅️ アプリケーションのエントリーポイントであることを示す

import (
	"git-gemini-reviewer-go/cli" // ⬅️ あなたの 'cli' パッケージをインポート
	"log"
)

func main() {
	// cli/root.go で定義された Execute 関数を呼び出す
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}
