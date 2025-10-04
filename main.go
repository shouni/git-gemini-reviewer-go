package main

import (
	"git-gemini-reviewer-go/cmd" // CLIのエントリポイント
)

func main() {
	// 埋め込み（embed）は cmd パッケージに移動したため、
	// main は単に cmd.Execute() を呼び出してアプリケーションを起動します。
	cmd.Execute()
}