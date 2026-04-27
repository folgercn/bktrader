package ctlclient

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEEvent 代表一个 SSE 事件
type SSEEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
	ID    string `json:"id"`
}

// StreamSSE 发起 SSE 请求并流式处理事件
func (c *Client) StreamSSE(method, path string, handler func(SSEEvent)) error {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	req, err := http.NewRequest(method, c.BaseURL+path, nil)
	if err != nil {
		return err
	}

	c.signRequest(req, nil)
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

	reader := bufio.NewReaderSize(resp.Body, 1024*1024)
	var currentEvent SSEEvent
	var dataBuffer bytes.Buffer

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			if dataBuffer.Len() > 0 {
				currentEvent.Data = dataBuffer.String()
				handler(currentEvent)
				dataBuffer.Reset()
				currentEvent = SSEEvent{}
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		key := parts[0]
		value := ""
		if len(parts) > 1 {
			value = strings.TrimSpace(parts[1])
		}

		switch key {
		case "event":
			currentEvent.Event = value
		case "data":
			if dataBuffer.Len() > 0 {
				dataBuffer.WriteByte('\n')
			}
			dataBuffer.WriteString(value)
		case "id":
			currentEvent.ID = value
		}
	}

	return nil
}
