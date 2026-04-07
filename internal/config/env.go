package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadEnvFile 在读取平台配置前，尽量把 .env 文件中的键值对加载进进程环境。
// 已存在的环境变量不会被覆盖，避免干扰外部注入的正式配置。
func LoadEnvFile() error {
	candidates := []string{}
	if custom := strings.TrimSpace(os.Getenv("APP_ENV_FILE")); custom != "" {
		candidates = append(candidates, custom)
	}
	candidates = append(candidates, ".env")

	for _, path := range candidates {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := loadSimpleEnvFile(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		return nil
	}
	return nil
}

func loadSimpleEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}
