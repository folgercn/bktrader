package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

const (
	repoOwner = "folgercn"
	repoName  = "bktrader"
	checkInterval = 6 * time.Hour // 每 6 小时检查一次
)

// SilentUpdateCheck 供 root 命令调用，执行静默检查和自动升级
func SilentUpdateCheck() {
	config, err := ctlclient.LoadConfig()
	if err == nil {
		if time.Since(config.LastUpdateCheck) < checkInterval {
			return
		}
	}

	// 执行检查
	latestTag, err := getLatestTag()
	if err != nil {
		return // 静默失败，不干扰主流程
	}

	// 更新检查时间
	if config != nil {
		config.LastUpdateCheck = time.Now()
		_ = ctlclient.SaveConfig(config)
	}

	if latestTag == "v"+Version || latestTag == Version {
		return
	}

	// 发现新版本，静默下载并替换
	downloadURL := getBinaryURL(latestTag)
	_ = downloadAndReplace(downloadURL)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "检查并更新 bktrader-ctl [MUTATING]",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("当前版本: %s\n", Version)
		fmt.Println("正在手动检查更新...")
		
		latestTag, err := getLatestTag()
		if err != nil {
			return fmt.Errorf("检查更新失败: %w", err)
		}

		if latestTag == "v"+Version || latestTag == Version {
			fmt.Println("✨ 您已经是最新版本。")
			return nil
		}

		fmt.Printf("发现新版本: %s，准备更新...\n", latestTag)
		if err := downloadAndReplace(getBinaryURL(latestTag)); err != nil {
			return err
		}
		fmt.Println("✅ 更新成功！")
		return nil
	},
}

func getLatestTag() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second) // 极短超时，保证不卡顿
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal([]byte(readAll(resp.Body)), &data); err != nil {
		return "", err
	}
	return data.TagName, nil
}

func readAll(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

func getBinaryURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/bktrader-ctl-%s-%s", 
		repoOwner, repoName, tag, runtime.GOOS, runtime.GOARCH)
}

func downloadAndReplace(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode != 200 { return fmt.Errorf("HTTP %d", resp.StatusCode) }

	exePath, err := os.Executable()
	if err != nil { return err }
	
	tmpPath := exePath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil { return err }
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil { return err }

	return os.Rename(tmpPath, exePath)
}
