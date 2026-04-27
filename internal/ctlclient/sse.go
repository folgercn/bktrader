package ctlclient

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Stream 发起 SSE 请求并流式处理事件 (支持多行数据和 1MB Buffer)
func (c *Client) Stream(method, path string, payload any, callback func([]byte)) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var body io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		return err
	}

	c.signRequest(req, payload)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("SSE request failed with status %d", resp.StatusCode)
	}

	// 使用 1MB 的 Reader 规避默认 Scanner 的 64K 限制
	reader := bufio.NewReaderSize(resp.Body, 1024*1024)
	var dataBuffer bytes.Buffer

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// 去掉换行符
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// SSE 协议中，空行表示一个事件的结束
		if line == "" {
			if dataBuffer.Len() > 0 {
				callback(dataBuffer.Bytes())
				dataBuffer.Reset()
			}
			continue
		}

		// 处理 data 字段，支持多行聚合
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimPrefix(line, "data:")
			data = strings.TrimSpace(data)
			dataBuffer.WriteString(data)
		}
		// 忽略 event:, id:, retry: 和注释行
	}

	return nil
}
