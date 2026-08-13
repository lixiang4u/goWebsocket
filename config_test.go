package goWebsocket

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const (
	expectedReadLimitBytes = 64 << 10
	expectedQueueSize      = 64
)

type synchronizedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func captureGlobalLogger(t *testing.T) *synchronizedBuffer {
	t.Helper()

	output := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	buffer := new(synchronizedBuffer)
	log.SetOutput(buffer)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(output)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
	return buffer
}

func websocketClient(t *testing.T, manager *WebsocketManager) *websocket.Conn {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	t.Cleanup(server.Close)

	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	return connection
}

func waitForLog(t *testing.T, buffer *synchronizedBuffer, contains string) string {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		output := buffer.String()
		if strings.Contains(output, contains) {
			return output
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log output did not contain %q; got %q", contains, buffer.String())
	return ""
}

func TestManagerConfigDefaults(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{})
	want := ManagerConfig{
		ReadLimitBytes:    expectedReadLimitBytes,
		OutboundQueueSize: expectedQueueSize,
		EventQueueSize:    expectedQueueSize,
	}
	if manager.Config != want {
		t.Fatalf("config = %#v, want %#v", manager.Config, want)
	}

	for name, queue := range map[string]chan EventCtx{
		"register":    manager.register,
		"bind":        manager.bind,
		"unbind":      manager.unbind,
		"join":        manager.join,
		"leave":       manager.leave,
		"send":        manager.send,
		"sendToGroup": manager.sendToGroup,
		"sendToUid":   manager.sendToUid,
		"broadcast":   manager.broadcast,
	} {
		if got := cap(queue); got != 0 {
			t.Errorf("%s queue capacity = %d, want unbuffered", name, got)
		}
	}
	if got := cap(manager.unregister); got != 0 {
		t.Errorf("unregister queue capacity = %d, want unbuffered", got)
	}
}

func TestManagerConfigNormalizesNonPositiveQueueAndLimitValues(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{
		Debug:             true,
		LogPayload:        true,
		ReadLimitBytes:    -1,
		OutboundQueueSize: 0,
		EventQueueSize:    -1,
	})

	if !manager.Config.Debug || !manager.Config.LogPayload {
		t.Fatalf("boolean config = %#v, want Debug and LogPayload enabled", manager.Config)
	}
	if manager.Config.ReadLimitBytes != expectedReadLimitBytes {
		t.Errorf("ReadLimitBytes = %d, want %d", manager.Config.ReadLimitBytes, expectedReadLimitBytes)
	}
	if manager.Config.OutboundQueueSize != expectedQueueSize {
		t.Errorf("OutboundQueueSize = %d, want %d", manager.Config.OutboundQueueSize, expectedQueueSize)
	}
	if manager.Config.EventQueueSize != expectedQueueSize {
		t.Errorf("EventQueueSize = %d, want %d", manager.Config.EventQueueSize, expectedQueueSize)
	}
}

func TestManagerConfigLegacyConstructorOnlyMapsDebug(t *testing.T) {
	manager := NewWebsocketManager(true)
	if !manager.Config.Debug {
		t.Fatal("legacy debug argument did not enable Debug")
	}
	if manager.Config.LogPayload {
		t.Fatal("legacy debug argument enabled LogPayload")
	}
	if manager.Config.ReadLimitBytes != expectedReadLimitBytes || manager.Config.OutboundQueueSize != expectedQueueSize || manager.Config.EventQueueSize != expectedQueueSize {
		t.Fatalf("legacy constructor config = %#v, want normalized defaults", manager.Config)
	}
}

func TestManagerConfigAppliesReadLimitPerInstance(t *testing.T) {
	const smallLimit = 64
	const largeLimit = 256
	payload := []byte(`{"event":"limit","data":"` + strings.Repeat("x", 80) + `"}`)
	if len(payload) <= smallLimit || len(payload) >= largeLimit {
		t.Fatalf("payload length = %d, want between %d and %d", len(payload), smallLimit, largeLimit)
	}

	smallManager := NewWebsocketManagerWithConfig(ManagerConfig{ReadLimitBytes: smallLimit})
	smallConnection := websocketClient(t, smallManager)
	if err := smallConnection.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write over-limit message: %v", err)
	}
	_ = smallConnection.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := smallConnection.ReadMessage(); err == nil {
		t.Fatal("over-limit connection remained open")
	}

	largeManager := NewWebsocketManagerWithConfig(ManagerConfig{ReadLimitBytes: largeLimit})
	handled := make(chan struct{}, 1)
	largeManager.On("limit", func(EventCtx) bool {
		handled <- struct{}{}
		return true
	})
	largeConnection := websocketClient(t, largeManager)
	if err := largeConnection.WriteMessage(websocket.TextMessage, payload); err != nil {
		t.Fatalf("write accepted message: %v", err)
	}
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("large-limit manager did not dispatch the same frame")
	}
}

func TestExampleSocketDoesNotLogRawEventContext(t *testing.T) {
	source, err := os.ReadFile("_examples/socket.go")
	if err != nil {
		t.Fatalf("read example socket source: %v", err)
	}
	if strings.Contains(string(source), "JsonString(ctx)") {
		t.Fatal("example socket logs JsonString(ctx), which can include raw event data")
	}
}

func TestExampleSocketLogsOnlyStaticCategories(t *testing.T) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "_examples/socket.go", nil, 0)
	if err != nil {
		t.Fatalf("parse example socket source: %v", err)
	}

	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isLogPrintCall(call) {
			return true
		}
		if len(call.Args) != 1 {
			t.Errorf("%s: log.Print* has %d arguments, want one static category", fileSet.Position(call.Pos()), len(call.Args))
			return true
		}
		if _, ok := call.Args[0].(*ast.BasicLit); !ok {
			t.Errorf("%s: log.Print* argument must be a static category", fileSet.Position(call.Args[0].Pos()))
		}
		return true
	})
}

func isLogPrintCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !strings.HasPrefix(selector.Sel.Name, "Print") {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "log"
}

func TestPayloadLoggingRedactsInboundAndProtocolErrorWithoutOptIn(t *testing.T) {
	buffer := captureGlobalLogger(t)
	manager := NewWebsocketManagerWithConfig(ManagerConfig{Debug: true})
	connection := websocketClient(t, manager)

	const inboundPayload = `{"event":"message","token":"inbound-secret"}`
	if err := connection.WriteMessage(websocket.TextMessage, []byte(inboundPayload)); err != nil {
		t.Fatalf("write inbound message: %v", err)
	}
	output := waitForLog(t, buffer, "[WebsocketRequest]")
	if strings.Contains(output, inboundPayload) || strings.Contains(output, "inbound-secret") || strings.Contains(output, "token") {
		t.Fatalf("payload appeared without opt-in: %q", output)
	}

	const invalidPayload = `{"credential":"protocol-secret"`
	if err := connection.WriteMessage(websocket.TextMessage, []byte(invalidPayload)); err != nil {
		t.Fatalf("write invalid message: %v", err)
	}
	output = waitForLog(t, buffer, "[WebsocketRequestProtocolError]")
	if strings.Contains(output, invalidPayload) || strings.Contains(output, "protocol-secret") || strings.Contains(output, "credential") {
		t.Fatalf("protocol payload appeared without opt-in: %q", output)
	}
}

func TestPayloadLoggingIncludesInboundPayloadOnlyWithOptIn(t *testing.T) {
	buffer := captureGlobalLogger(t)
	manager := NewWebsocketManagerWithConfig(ManagerConfig{Debug: true, LogPayload: true})
	connection := websocketClient(t, manager)

	const payload = `{"event":"message","token":"opt-in-secret"}`
	if err := connection.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
		t.Fatalf("write inbound message: %v", err)
	}
	output := waitForLog(t, buffer, "[WebsocketRequest]")
	if !strings.Contains(output, payload) {
		t.Fatalf("payload was absent with explicit opt-in: %q", output)
	}
}

func TestPayloadLoggingIsDisabledWhenDebugIsFalse(t *testing.T) {
	buffer := captureGlobalLogger(t)
	manager := NewWebsocketManagerWithConfig(ManagerConfig{LogPayload: true})
	handled := make(chan struct{}, 1)
	manager.On("message", func(EventCtx) bool {
		handled <- struct{}{}
		return true
	})
	connection := websocketClient(t, manager)

	if err := connection.WriteMessage(websocket.TextMessage, []byte(`{"event":"message","token":"hidden"}`)); err != nil {
		t.Fatalf("write inbound message: %v", err)
	}
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("inbound message was not dispatched")
	}
	if output := buffer.String(); output != "" {
		t.Fatalf("Debug=false emitted log output: %q", output)
	}
}
