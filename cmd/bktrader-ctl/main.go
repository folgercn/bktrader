package main

// 这些变量将在构建时通过 -ldflags 注入
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func main() {
	Execute()
}
