package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/http"
)

func main() {
	fmt.Println("Checking CLI command coverage...")

	// 1. 获取所有 CLI 代码文件内容
	cliDir := "cmd/bktrader-ctl"
	var allCliContent string
	err := filepath.Walk(cliDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		content, _ := ioutil.ReadFile(path)
		allCliContent += string(content)
		return nil
	})
	if err != nil {
		fmt.Printf("Error walking CLI dir: %v\n", err)
		os.Exit(1)
	}

	// 2. 检查 APIRegistry 中的每一项
	missingCount := 0
	for _, entry := range http.APIRegistry {
		if entry.CLICommand == "" {
			continue
		}
		// 寻找 Use: "command" 或类似的注册模式
		// 简单起见，这里检查 CLICommand 的最后一个单词是否出现在代码中
		parts := strings.Fields(entry.CLICommand)
		lastPart := parts[len(parts)-1]

		// 更加严格的检查：搜索 Use: "command" 或 Use: "command <arg>"
		searchStrNormal := fmt.Sprintf(`Use:   "%s"`, lastPart)
		searchStrWithArgs := fmt.Sprintf(`Use:   "%s `, lastPart) // 后面带空格，说明有参数
		if !strings.Contains(allCliContent, searchStrNormal) && !strings.Contains(allCliContent, searchStrWithArgs) {
			fmt.Printf("❌ Missing CLI command for API: %s %s (Expected: %s)\n",
				strings.Join(entry.Methods, ","), entry.Path, entry.CLICommand)
			missingCount++
		}
	}

	if missingCount > 0 {
		fmt.Printf("\nTotal missing: %d\n", missingCount)
		fmt.Println("Please run 'go run scripts/gen-ctl-command.go' to generate skeleton code.")
		os.Exit(1)
	}

	fmt.Println("✅ All registered APIs are covered by CLI commands.")
}
