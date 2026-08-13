package goWebsocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketIntegrationGroupEnqueueAndManagerClose(t *testing.T) {
	manager := NewWebsocketManager()
	connected := make(chan string, 1)
	joined := make(chan struct{}, 1)
	manager.On(Event(EventConnect).String(), func(ctx EventCtx) bool {
		connected <- ctx.From
		return true
	})
	manager.On(Event(EventJoinGroup).String(), func(EventCtx) bool {
		joined <- struct{}{}
		return true
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	var clientID string
	select {
	case clientID = <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("connect event timeout")
	}
	manager.JoinGroup(clientID, "room")
	receiveWithin(t, joined)

	result := manager.EnqueueToGroup("room", map[string]string{"event": "video"})
	if len(result.Accepted) != 1 || result.Accepted[0] != clientID || len(result.Rejected) != 0 {
		t.Fatalf("enqueue result = %#v", result)
	}
	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, payload, err := client.ReadMessage()
	if err != nil {
		t.Fatalf("read group payload: %v", err)
	}
	if string(payload) != `{"event":"video"}` {
		t.Fatalf("payload = %q", payload)
	}

	if err := manager.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	_ = client.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, _, err := client.ReadMessage(); err == nil {
		t.Fatal("transport remained readable after manager Close")
	}
}
