package ctlclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStreamSSEJoinsMultiLineDataWithNewline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: log\nid: 1\ndata: first\ndata: second\n\n"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	var got []SSEEvent
	err := client.StreamSSE("GET", "/events", func(ev SSEEvent) {
		got = append(got, ev)
	})
	if err != nil {
		t.Fatalf("StreamSSE returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Data != "first\nsecond" {
		t.Fatalf("expected multi-line data to be joined with newline, got %q", got[0].Data)
	}
}
