package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

var (
	cfgFile    string
	outputJSON bool
	dryRun     bool
	apiURL     string
	apiToken   string
)

var rootCmd = &cobra.Command{
	Use:   "bktrader-ctl",
	Short: "bktrader 远程控制命令行工具",
	Long: `bktrader-ctl 是一个 LLM/Agent-first 的远程控制工具，
主要用于管理 bktrader platform-api 实例。

支持环境变量配置:
  BKTRADER_API_URL    API 地址 (默认 http://localhost:8080)
  BKTRADER_TOKEN      Bearer Token
  BKTRADER_USERNAME   登录用户名
  BKTRADER_PASSWORD   登录密码`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig()
	},
}

// Execute 执行根命令
func Execute() {
	// 静默更新检查失败不阻塞主命令执行。
	go SilentUpdateCheck()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.bktrader-ctl.yaml)")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "输出 JSON 格式 (LLM 推荐)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "预览操作而不实际发送 (Mutating commands only)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "API 基础地址 (e.g. http://localhost:8080)")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API 鉴权 Token")

	_ = viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".bktrader-ctl")
	}

	viper.SetEnvPrefix("BKTRADER")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func initializeConfig() error {
	// 补足默认值
	if viper.GetString("api_url") == "" {
		viper.Set("api_url", "http://localhost:8080")
	}

	// 如果 flag 没传 token，尝试从缓存加载
	if viper.GetString("token") == "" {
		if cached, err := ctlclient.LoadToken(); err == nil {
			viper.Set("token", cached)
		}
	}
	return nil
}

func getClient() *ctlclient.Client {
	c := ctlclient.NewClient(viper.GetString("api_url"), viper.GetString("token"))
	c.DryRun = dryRun
	return c
}

func handleResponse(data []byte, err error) {
	if err != nil {
		exitCode := 1 // 默认通用错误

		// 根据 API 错误类型区分退出码 (参见 LLM Guide §1)
		if apiErr, ok := err.(*ctlclient.APIError); ok {
			switch {
			case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
				exitCode = 3 // 鉴权失败
			case apiErr.StatusCode == 429 || apiErr.StatusCode == 409:
				exitCode = 2 // 业务逻辑错误
			}
		}

		if outputJSON {
			fmt.Printf(`{"status":"error","message":%q,"exit_code":%d}`+"\n", err.Error(), exitCode)
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(exitCode)
	}

	if outputJSON {
		fmt.Println(string(data))
	} else {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "  "); err == nil {
			fmt.Println(pretty.String())
		} else {
			fmt.Println(string(data))
		}
	}
}
