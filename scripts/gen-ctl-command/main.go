package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	route := flag.String("route", "", "API 路由路径 (e.g. /api/v1/orders)")
	method := flag.String("method", "GET", "HTTP 方法 (GET, POST, DELETE)")
	cmdName := flag.String("cmd", "", "CLI 命令名称 (e.g. order-list)")
	flag.Parse()

	if *route == "" || *cmdName == "" {
		fmt.Println("Usage: go run scripts/gen-ctl-command.go --route /api/v1/path --method GET --cmd my-cmd")
		os.Exit(1)
	}

	tmpl := `
var %sCmd = &cobra.Command{
	Use:   "%s",
	Short: "简短描述 [%s]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("%s", "%s", nil)
		handleResponse(resp, err)
		return nil
	},
}
`
	// 转换为 camelCase 用于变量名
	varName := strings.ReplaceAll(*cmdName, "-", "")
	varName = strings.ReplaceAll(varName, "_", "")
	
	idempotent := "MUTATING"
	if *method == "GET" {
		idempotent = "IDEMPOTENT"
	}

	fmt.Printf("--- Generated Code Skeleton ---\n")
	fmt.Printf(tmpl, varName, *cmdName, idempotent, *method, *route)
	fmt.Printf("\nRemember to:\n1. Add flags in init()\n2. Register command in init()\n3. Update APIRegistry in internal/http/registry.go\n")
}
