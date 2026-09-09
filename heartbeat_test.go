package goWebsocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
)

// A heartbeat must not close or corrupt an in-progress application message.
// This intentionally uses the production heartbeat interval (45 seconds).
func TestHeartbeatPreservesApplicationWriter(t *testing.T) {
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		connections <- conn
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	conn := <-connections
	defer conn.Close()
	manager := &WebsocketManager{clients: cmap.New[ConnectionCtx]()}
	manager.clients.Set("test", ConnectionCtx{Socket: conn})
	defer manager.clients.Remove("test")

	writer, err := conn.NextWriter(websocket.TextMessage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("before-")); err != nil {
		t.Fatal(err)
	}
	ping := make(chan struct{}, 1)
	client.SetPingHandler(func(string) error {
		select {
		case ping <- struct{}{}:
		default:
		}
		return nil
	})
	type result struct {
		data string
		err  error
	}
	messages := make(chan result, 2)
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			_, data, err := client.ReadMessage()
			messages <- result{string(data), err}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		client.Close()
		<-readerDone
	}()
	go manager.writeMessage("test", conn)
	select {
	case <-ping:
	case <-time.After(writeTicker + 10*time.Second):
		t.Fatal("heartbeat not received")
	}
	if _, err := writer.Write([]byte("after")); err != nil {
		t.Fatalf("heartbeat invalidated the application writer: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-messages:
		if got.err != nil || got.data != "before-after" {
			t.Fatalf("message = %q, err = %v; want intact before-after", got.data, got.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("application message not received")
	}
}
