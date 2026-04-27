package ctlclient

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"
)

// SSEEvent 代表一个 SSE 事件
type SSEEvent struct {
	Event string
	Data  string
}

// StreamSSE 发起 SSE 请求并流式处理事件
func (c *Client) StreamSSE(method, path string, handler func(SSEEvent)) error {
	url := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	var currentEvent SSEEvent

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// 空行表示事件结束，分发事件
			if currentEvent.Data != "" {
				handler(currentEvent)
			}
			currentEvent = SSEEvent{}
			continue
		}

		if strings.HasPrefix(line, ":") {
			// 注释行/keepalive
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "event":
			currentEvent.Event = value
		case "data":
			currentEvent.Data = value
		}
	}

	return scanner.Err()
}
