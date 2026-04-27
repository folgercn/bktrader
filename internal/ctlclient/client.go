package ctlclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Client 是 bktrader-ctl 的 API 客户端
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	DryRun     bool
}

// Config 存储本地 token 和配置
type Config struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewClient(baseURL, token string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Request 执行 HTTP 请求
func (c *Client) Request(method, path string, body any) ([]byte, error) {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body failed: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonBody)
		if c.DryRun {
			fmt.Fprintf(os.Stderr, "[DRY RUN] %s %s\nPayload: %s\n", method, url, string(jsonBody))
		}
	} else if c.DryRun {
		fmt.Fprintf(os.Stderr, "[DRY RUN] %s %s\n", method, url)
	}

	if c.DryRun && method != http.MethodGet {
		return []byte(`{"status":"dry-run","message":"request not sent"}`), nil
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		return respBody, &APIError{
			StatusCode: resp.StatusCode,
			RawMessage: string(respBody),
		}
	}

	return respBody, nil
}

// APIError 代表结构化 API 错误
type APIError struct {
	StatusCode int
	RawMessage string
}

func (e *APIError) Error() string {
	var data struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(e.RawMessage), &data); err == nil && data.Error != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, data.Error)
	}
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.RawMessage)
}

// GetTokenCachePath 返回本地 token 缓存路径
func GetTokenCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bktrader-ctl", "token.json")
}

// LoadToken 从本地加载缓存的 token
func LoadToken() (string, error) {
	path := GetTokenCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	if time.Now().After(cfg.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return cfg.Token, nil
}

// SaveToken 保存 token 到本地
func SaveToken(token string, ttl time.Duration) error {
	path := GetTokenCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	cfg := Config{
		Token:     token,
		ExpiresAt: time.Now().Add(ttl),
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
