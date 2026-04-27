package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wuyaocheng/bktrader/internal/ctlclient"
)

func init() {
	rootCmd.AddCommand(updateCmd)
}

const (
	repoOwner     = "folgercn"
	repoName      = "bktrader"
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

	// 并发锁：防止多个命令同时触发更新导致文件冲突
	lockPath := filepath.Join(os.TempDir(), "bktrader-ctl-update.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		// 如果锁文件已存在且未超过 5 分钟，说明已有更新在运行
		if info, err := os.Stat(lockPath); err == nil && time.Since(info.ModTime()) < 5*time.Minute {
			return
		}
		// 否则尝试清理旧锁 (强制接管)
		_ = os.Remove(lockPath)
		lockFile, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL, 0600)
		if err != nil { return }
	}
	defer func() {
		lockFile.Close()
		_ = os.Remove(lockPath)
	}()

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
	_ = downloadAndReplace(latestTag, downloadURL)
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
		if err := downloadAndReplace(latestTag, getBinaryURL(latestTag)); err != nil {
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
	if err != nil {
		return "", err
	}
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

func getChecksumURL(tag string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/checksums.txt",
		repoOwner, repoName, tag)
}

func downloadAndReplace(tag, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. 获取预期的哈希值
	expectedHash, err := getExpectedHash(ctx, tag)
	if err != nil {
		return fmt.Errorf("无法获取校验和: %w", err)
	}

	// 2. 发起下载请求
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 尺寸预检
	if resp.ContentLength <= 0 {
		return fmt.Errorf("异常的 Content-Length: %d", resp.ContentLength)
	}

	// 3. 准备临时文件
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	tmpPath := exePath + ".next"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}

	// 4. 流式下载并计算哈希
	hasher := sha256.New()
	multiWriter := io.MultiWriter(f, hasher)

	copied, err := io.Copy(multiWriter, resp.Body)
	if err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	if copied != resp.ContentLength && resp.ContentLength > 0 {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("下载不完整: expected %d, got %d", resp.ContentLength, copied)
	}

	// 5. 校验哈希
	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("校验失败! 预期: %s, 实际: %s", expectedHash, actualHash)
	}

	// 6. 确保落盘并关闭
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	f.Close()

	// 7. 原子替换策略 (Unix-friendly)
	// 先将当前的重命名为 .old，再把 .next 换过来
	oldPath := exePath + ".old"
	_ = os.Remove(oldPath) // 清理上次的备份

	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("备份当前程序失败: %w", err)
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		// 尝试回滚
		_ = os.Rename(oldPath, exePath)
		return fmt.Errorf("替换新版本失败: %w", err)
	}

	return nil
}

func getExpectedHash(ctx context.Context, tag string) (string, error) {
	url := getChecksumURL(tag)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	lines := strings.Split(string(body), "\n")

	targetName := fmt.Sprintf("bktrader-ctl-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == targetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("在 checksums.txt 中未找到 %s", targetName)
}
