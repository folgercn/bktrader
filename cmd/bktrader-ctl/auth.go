package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authMeCmd)
	rootCmd.AddCommand(authCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "鉴权管理",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "登录并获取 Token [MUTATING]",
	RunE: func(cmd *cobra.Command, args []string) error {
		username := viper.GetString("username")
		password := viper.GetString("password")

		if username == "" || password == "" {
			return fmt.Errorf("请通过环境变量 BKTRADER_USERNAME 和 BKTRADER_PASSWORD 提供登录凭证")
		}

		client := getClient()
		payload := map[string]string{
			"username": username,
			"password": password,
		}

		resp, err := client.Request("POST", "/api/v1/auth/login", payload)
		if err != nil {
			return err
		}

		var data struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(resp, &data); err != nil {
			return fmt.Errorf("解析登录响应失败: %w", err)
		}

		// 缓存 Token，假设 TTL 为 24 小时
		if err := ctlclient.SaveToken(data.Token, 24*time.Hour); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 无法保存 Token 缓存: %v\n", err)
		}

		if outputJSON {
			fmt.Printf(`{"status":"ok","token":%q}`+"\n", data.Token)
		} else {
			fmt.Println("登录成功！Token 已缓存至 ~/.bktrader-ctl/token.json")
		}
		return nil
	},
}

var authMeCmd = &cobra.Command{
	Use:   "me",
	Short: "检查当前身份 [IDEMPOTENT]",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := getClient()
		resp, err := client.Request("GET", "/api/v1/auth/me", nil)
		handleResponse(resp, err)
		return nil
	},
}
