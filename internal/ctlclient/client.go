package ctlclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type RouteEntry struct {
	Path        string
	Methods     []string
	Module      string
	Description string
	CLICommand  string
	Idempotent  bool
	RiskLevel   string
}

type Client struct {
	BaseURL string
	Token   string
	DryRun  bool
}

type Config struct {
	Token           string    `json:"token"`
	ExpiresAt       time.Time `json:"expiresAt"`
	LastUpdateCheck time.Time `json:"lastUpdateCheck"`
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
	}
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
}

// Request 发起同步 HTTP 请求
func (c *Client) Request(method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	c.signRequest(req, payload)

	if c.DryRun && (method == "POST" || method == "PUT" || method == "DELETE") {
		fmt.Fprintf(os.Stderr, "[Dry-run] %s %s\n", method, url)
		return []byte(`{"status":"dry-run"}`), nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(data),
		}
	}

	return data, nil
}

// signRequest 处理鉴权和签名
func (c *Client) signRequest(req *http.Request, _ any) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Content-Type", "application/json")
}

func GetTokenCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".bktrader-ctl.cache.json")
}

func LoadConfig() (*Config, error) {
	path := GetTokenCachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := GetTokenCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func SaveToken(token string, expiresAt time.Time) error {
	config, err := LoadConfig()
	if err != nil {
		config = &Config{}
	}
	config.Token = token
	config.ExpiresAt = expiresAt
	return SaveConfig(config)
}

func LoadToken() (string, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return "", err
	}
	if time.Now().After(cfg.ExpiresAt) {
		return "", fmt.Errorf("token expired")
	}
	return cfg.Token, nil
}
