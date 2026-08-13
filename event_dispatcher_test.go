package goWebsocket

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestEventQueueFullLogsFixedCategoryOnceForManagerEventPath(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{Debug: true, EventQueueSize: 1})
	defer manager.Close()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	worker := manager.eventDispatcher.replaceClient("sensitive-client")
	manager.connectionRecords.Set("sensitive-client", connectionRecord{worker: worker, ready: true})
	manager.clients.Set("sensitive-client", ConnectionCtx{Group: make(map[string]bool)})
	started := make(chan struct{})
	release := make(chan struct{})
	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(started)
		<-release
		return true
	}}) {
		t.Fatal("blocking task rejected")
	}
	receiveWithin(t, started)
	if !worker.submit(userEventTask{handler: func(EventCtx) bool { return true }}) {
		t.Fatal("queued task rejected")
	}
	manager.On(Event(EventBindUid).String(), func(EventCtx) bool { return true })
	manager.BindUid("sensitive-client", "sensitive-uid")
	manager.JoinGroup("sensitive-client", "sensitive-group")
	close(release)
	receiveWithin(t, worker.done)

	logged := output.String()
	if strings.Count(logged, "[WebsocketEventQueueFull]") != 1 {
		t.Fatalf("queue-full log count = %d, log=%q", strings.Count(logged, "[WebsocketEventQueueFull]"), logged)
	}
	for _, sensitive := range []string{"sensitive-client", "sensitive-uid", "sensitive-group"} {
		if strings.Contains(logged, sensitive) {
			t.Fatalf("queue-full log leaked %q: %q", sensitive, logged)
		}
	}
}

func TestEventQueueFullConcurrentFailuresReportOncePerWorkerIdentity(t *testing.T) {
	var count atomic.Int32
	dispatcher := newEventDispatcher(1, nil, func() { count.Add(1) })
	worker := dispatcher.replaceClient("same-instance")
	started := make(chan struct{})
	release := make(chan struct{})
	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(started)
		<-release
		return true
	}}) {
		t.Fatal("blocking task rejected")
	}
	receiveWithin(t, started)
	if !worker.submit(userEventTask{}) {
		t.Fatal("queue filler rejected")
	}

	const attempts = 16
	var attemptsDone sync.WaitGroup
	attemptsDone.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func() {
			defer attemptsDone.Done()
			if worker.submit(userEventTask{}) {
				t.Errorf("full queue accepted concurrent task")
			}
		}()
	}
	attemptsDone.Wait()
	if count.Load() != 1 {
		t.Fatalf("queue-full reports = %d, want 1", count.Load())
	}
	close(release)
	worker.submit(userEventTask{terminal: true})
	receiveWithin(t, worker.done)

	replacement := dispatcher.replaceClient("same-instance")
	blockReplacement := make(chan struct{})
	replacementStarted := make(chan struct{})
	if !replacement.submit(userEventTask{handler: func(EventCtx) bool {
		close(replacementStarted)
		<-blockReplacement
		return true
	}}) {
		t.Fatal("replacement blocking task rejected")
	}
	receiveWithin(t, replacementStarted)
	if !replacement.submit(userEventTask{}) {
		t.Fatal("replacement queue filler rejected")
	}
	if replacement.submit(userEventTask{}) {
		t.Fatal("replacement full queue accepted task")
	}
	if count.Load() != 2 {
		t.Fatalf("queue-full reports after replacement = %d, want 2", count.Load())
	}
	close(blockReplacement)
	replacement.submit(userEventTask{terminal: true})
	receiveWithin(t, replacement.done)
}

func TestEventQueueFullLogsFixedCategoryOnceForReadPath(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{Debug: true, EventQueueSize: 1})
	defer manager.Close()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	serverSocket, clientSocket := websocketTestPair(t)
	defer clientSocket.Close()
	worker := manager.registerConnectionSync(EventCtx{
		From:   "read-sensitive",
		Socket: serverSocket,
		FromCtx: &FromCtx{
			UserAgent: "sensitive-agent",
		},
	})
	started := make(chan struct{})
	release := make(chan struct{})
	manager.On("read-event", func(EventCtx) bool {
		close(started)
		<-release
		return true
	})
	readDone := make(chan struct{})
	go func() {
		manager.readMessage("read-sensitive", serverSocket, worker)
		close(readDone)
	}()
	if err := clientSocket.WriteJSON(EventCtx{Event: "read-event", Data: "payload-secret"}); err != nil {
		t.Fatalf("write first read event: %v", err)
	}
	receiveWithin(t, started)
	for i := 0; i < 2; i++ {
		if err := clientSocket.WriteJSON(EventCtx{Event: "read-event", Data: "payload-secret"}); err != nil {
			t.Fatalf("write queued read event %d: %v", i, err)
		}
	}
	receiveWithin(t, readDone)
	close(release)
	receiveWithin(t, worker.done)

	logged := output.String()
	if strings.Count(logged, "[WebsocketEventQueueFull]") != 1 {
		t.Fatalf("queue-full log count = %d, log=%q", strings.Count(logged, "[WebsocketEventQueueFull]"), logged)
	}
	for _, sensitive := range []string{"read-sensitive", "payload-secret", "sensitive-agent"} {
		if strings.Contains(logged, sensitive) {
			t.Fatalf("queue-full log leaked %q: %q", sensitive, logged)
		}
	}
}

func TestEventDispatcherPreservesClientFIFOAndTerminal(t *testing.T) {
	dispatcher := newEventDispatcher(4, func() {})
	worker := dispatcher.replaceClient("client-a")

	var mu sync.Mutex
	got := make([]string, 0, 4)
	appendEvent := func(name string) EventHandler {
		return func(EventCtx) bool {
			mu.Lock()
			got = append(got, name)
			mu.Unlock()
			return true
		}
	}

	for _, task := range []userEventTask{
		{handler: appendEvent("connect")},
		{handler: appendEvent("a")},
		{handler: appendEvent("b")},
		{handler: appendEvent("close"), terminal: true},
	} {
		if !worker.submit(task) {
			t.Fatalf("submit %#v failed", task)
		}
	}
	receiveWithin(t, worker.done)

	mu.Lock()
	defer mu.Unlock()
	if want := []string{"connect", "a", "b", "close"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
	if worker.submit(userEventTask{handler: appendEvent("late")}) {
		t.Fatal("worker accepted an event after terminal")
	}
}

func TestEventDispatcherRunsDifferentClientsConcurrently(t *testing.T) {
	dispatcher := newEventDispatcher(2, func() {})
	clientA := dispatcher.replaceClient("client-a")
	clientB := dispatcher.replaceClient("client-b")

	aStarted := make(chan struct{})
	releaseA := make(chan struct{})
	bRan := make(chan struct{})
	if !clientA.submit(userEventTask{handler: func(EventCtx) bool {
		close(aStarted)
		<-releaseA
		return true
	}}) {
		t.Fatal("submit client A failed")
	}
	receiveWithin(t, aStarted)
	if !clientB.submit(userEventTask{handler: func(EventCtx) bool {
		close(bRan)
		return true
	}}) {
		t.Fatal("submit client B failed")
	}
	receiveWithin(t, bRan)
	close(releaseA)
	if !clientA.submit(userEventTask{terminal: true}) || !clientB.submit(userEventTask{terminal: true}) {
		t.Fatal("terminal submit failed")
	}
	receiveWithin(t, clientA.done)
	receiveWithin(t, clientB.done)
}

func TestEventDispatcherRecoversHandlerPanic(t *testing.T) {
	panicLogged := make(chan struct{}, 1)
	dispatcher := newEventDispatcher(2, func() { panicLogged <- struct{}{} })
	worker := dispatcher.replaceClient("client-a")
	nextRan := make(chan struct{})

	if !worker.submit(userEventTask{handler: func(EventCtx) bool { panic("boom") }}) {
		t.Fatal("submit panic task failed")
	}
	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(nextRan)
		return true
	}}) {
		t.Fatal("submit next task failed")
	}
	if !worker.submit(userEventTask{terminal: true}) {
		t.Fatal("submit terminal failed")
	}
	receiveWithin(t, panicLogged)
	receiveWithin(t, nextRan)
	receiveWithin(t, worker.done)
}

func TestEventDispatcherQueueFullIsNonBlockingAndTerminalStillRuns(t *testing.T) {
	dispatcher := newEventDispatcher(1, func() {})
	worker := dispatcher.replaceClient("client-a")
	started := make(chan struct{})
	release := make(chan struct{})
	closed := make(chan struct{})

	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(started)
		<-release
		return true
	}}) {
		t.Fatal("submit blocking task failed")
	}
	receiveWithin(t, started)
	if !worker.submit(userEventTask{handler: func(EventCtx) bool { return true }}) {
		t.Fatal("submit queued task failed")
	}
	returned := make(chan bool, 1)
	go func() {
		returned <- worker.submit(userEventTask{handler: func(EventCtx) bool { return true }})
	}()
	if accepted := receiveWithin(t, returned); accepted {
		t.Fatal("full queue accepted ordinary task")
	}
	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(closed)
		return true
	}, terminal: true}) {
		t.Fatal("full queue rejected terminal task")
	}
	close(release)
	receiveWithin(t, closed)
	receiveWithin(t, worker.done)
}

func TestEventDispatcherOldTerminalCannotDeleteReplacement(t *testing.T) {
	dispatcher := newEventDispatcher(1, func() {})
	oldWorker := dispatcher.replaceClient("reused")
	newWorker := dispatcher.replaceClient("reused")

	if !oldWorker.submit(userEventTask{terminal: true}) {
		t.Fatal("old terminal submit failed")
	}
	receiveWithin(t, oldWorker.done)
	if got := dispatcher.currentClient("reused"); got != newWorker {
		t.Fatalf("current worker = %p, want replacement %p", got, newWorker)
	}
	if !newWorker.submit(userEventTask{terminal: true}) {
		t.Fatal("new terminal submit failed")
	}
	receiveWithin(t, newWorker.done)
}

func TestEventDispatcherOldOverflowCannotCloseReplacementSocket(t *testing.T) {
	oldServer, oldClient := websocketTestPair(t)
	newServer, newClient := websocketTestPair(t)
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 1})
	closedUID := make(chan string, 1)
	manager.On(Event(EventClose).String(), func(ctx EventCtx) bool {
		closedUID <- ctx.FromCtx.Uid
		return true
	})

	oldWorker := manager.eventDispatcher.replaceClient("reused", EventCtx{Socket: oldServer, FromCtx: &FromCtx{Uid: "old-uid"}})
	manager.connect(EventCtx{From: "reused", Socket: oldServer}, oldWorker)
	newWorker := manager.eventDispatcher.replaceClient("reused", EventCtx{Socket: newServer})
	manager.clients.Set("reused", ConnectionCtx{Socket: newServer, Group: make(map[string]bool), Uid: "new-uid"})
	manager.connectionRecords.Set("reused", connectionRecord{socket: newServer, worker: newWorker})

	manager.disconnectEventQueueOverflow("reused", oldWorker)
	receiveWithin(t, oldWorker.done)
	if uid := receiveWithin(t, closedUID); uid != "old-uid" {
		t.Fatalf("old close context uid = %q, want old-uid", uid)
	}
	if got := manager.eventDispatcher.currentClient("reused"); got != newWorker {
		t.Fatalf("replacement worker removed: got %p, want %p", got, newWorker)
	}
	if current, ok := manager.clients.Get("reused"); !ok || current.Socket != newServer {
		t.Fatal("replacement connection index was removed")
	}
	if err := newServer.WriteMessage(websocket.TextMessage, []byte("still-open")); err != nil {
		t.Fatalf("replacement socket was closed: %v", err)
	}
	_ = newClient.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := newClient.ReadMessage()
	if err != nil {
		t.Fatalf("read replacement socket: %v", err)
	}
	if string(payload) != "still-open" {
		t.Fatalf("replacement payload = %q", payload)
	}
	if err := oldServer.WriteMessage(websocket.TextMessage, []byte("closed")); err == nil {
		t.Fatal("old socket remained writable after overflow disconnect")
	}
	if !newWorker.submit(userEventTask{terminal: true}) {
		t.Fatal("new terminal submit failed")
	}
	receiveWithin(t, newWorker.done)
	_ = oldClient.Close()
}

func websocketTestPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConn := make(chan *websocket.Conn, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serverConn <- conn
		<-release
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket pair: %v", err)
	}
	serverSide := receiveWithin(t, serverConn)
	t.Cleanup(func() {
		_ = client.Close()
		_ = serverSide.Close()
		close(release)
		server.Close()
	})
	return serverSide, client
}

func TestClientEventFIFOCapturesHandlerAtSubmission(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	worker := manager.eventDispatcher.replaceClient("client-a")
	started := make(chan struct{})
	release := make(chan struct{})
	got := make(chan string, 1)

	if !worker.submit(userEventTask{handler: func(EventCtx) bool {
		close(started)
		<-release
		return true
	}}) {
		t.Fatal("submit barrier failed")
	}
	receiveWithin(t, started)
	manager.On("event", func(EventCtx) bool { got <- "old"; return true })
	if !manager.dispatchConnectionUserEvent(worker, "event", EventCtx{From: "client-a"}, false) {
		t.Fatal("dispatch failed")
	}
	manager.On("event", func(EventCtx) bool { got <- "new"; return true })
	if !worker.submit(userEventTask{terminal: true}) {
		t.Fatal("terminal submit failed")
	}
	close(release)
	if value := receiveWithin(t, got); value != "old" {
		t.Fatalf("captured handler = %q, want old", value)
	}
	receiveWithin(t, worker.done)
}

func TestClientEventFIFOHandlerWaitsForConnect(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	connectStarted := make(chan struct{})
	releaseConnect := make(chan struct{})
	messageRan := make(chan struct{})
	manager.On(Event(EventConnect).String(), func(EventCtx) bool {
		close(connectStarted)
		<-releaseConnect
		return true
	})
	manager.On("message", func(EventCtx) bool {
		close(messageRan)
		return true
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	defer server.Close()
	wsURL := "ws" + server.URL[len("http"):]
	client, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	receiveWithin(t, connectStarted)
	if err := client.WriteJSON(EventCtx{Event: "message"}); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case <-messageRan:
		t.Fatal("message handler ran before connect handler completed")
	default:
	}
	close(releaseConnect)
	receiveWithin(t, messageRan)
}

func TestClientEventFIFORealWebSocketConnectFramesAndClose(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 8})
	events := make(chan string, 4)
	clientID := make(chan string, 1)
	var closeCount atomic.Int32
	manager.On(Event(EventConnect).String(), func(ctx EventCtx) bool { clientID <- ctx.From; events <- "connect"; return true })
	manager.On("a", func(EventCtx) bool { events <- "a"; return true })
	manager.On("b", func(EventCtx) bool { events <- "b"; return true })
	manager.On(Event(EventClose).String(), func(EventCtx) bool { closeCount.Add(1); events <- "close"; return true })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := client.WriteJSON(EventCtx{Event: "a"}); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := client.WriteJSON(EventCtx{Event: "b"}); err != nil {
		t.Fatalf("write b: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	id := receiveWithin(t, clientID)
	got := []string{receiveWithin(t, events), receiveWithin(t, events), receiveWithin(t, events), receiveWithin(t, events)}
	if want := []string{"connect", "a", "b", "close"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("real websocket order = %v, want %v", got, want)
	}
	waitForCondition(t, func() bool { return manager.eventDispatcher.currentClient(id) == nil })
	if got := closeCount.Load(); got != 1 {
		t.Fatalf("close count = %d, want 1", got)
	}
}

func TestClientEventFIFOPresenceImmediatelyBeforeClose(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	events := make(chan string, 3)
	manager.On(Event(EventConnect).String(), func(EventCtx) bool { events <- "connect"; return true })
	manager.On("presence", func(EventCtx) bool { events <- "presence"; return true })
	manager.On(Event(EventClose).String(), func(EventCtx) bool { events <- "close"; return true })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := client.WriteJSON(EventCtx{Event: "presence"}); err != nil {
		t.Fatalf("write presence: %v", err)
	}
	_ = client.Close()
	got := []string{receiveWithin(t, events), receiveWithin(t, events), receiveWithin(t, events)}
	if want := []string{"connect", "presence", "close"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("presence/close order = %v, want %v", got, want)
	}
}

func TestClientEventFIFOReplacementAndLateDisconnectDoNotHarmNewInstance(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	oldServer, oldClient := websocketTestPair(t)
	newServer, newClient := websocketTestPair(t)
	closeEvents := make(chan string, 2)
	manager.On(Event(EventClose).String(), func(ctx EventCtx) bool {
		closeEvents <- ctx.FromCtx.UserAgent
		return true
	})

	oldCtx := EventCtx{From: "reused", Socket: oldServer, FromCtx: &FromCtx{UserAgent: "old-agent", RequestURI: "/old"}}
	oldWorker := manager.registerConnectionSync(oldCtx)
	manager.BindUid("reused", "old-uid")
	manager.JoinGroup("reused", "old-room")
	newCtx := EventCtx{From: "reused", Socket: newServer, FromCtx: &FromCtx{UserAgent: "new-agent", RequestURI: "/new"}}
	newWorker := manager.registerConnectionSync(newCtx)
	if oldWorker == newWorker {
		t.Fatal("replacement reused old worker")
	}
	if agent := receiveWithin(t, closeEvents); agent != "old-agent" {
		t.Fatalf("replacement close agent = %q, want old-agent", agent)
	}
	receiveWithin(t, oldWorker.done)
	if got := manager.eventDispatcher.currentClient("reused"); got != newWorker {
		t.Fatalf("old cleanup removed replacement worker: got %p, want %p", got, newWorker)
	}
	manager.Disconnect(EventCtx{From: "reused", Socket: oldServer})

	current, ok := manager.clients.Get("reused")
	if !ok || current.Socket != newServer || manager.currentClientWorker("reused") != newWorker {
		t.Fatal("late old disconnect removed or mismatched replacement")
	}
	if err := newServer.WriteMessage(websocket.TextMessage, []byte("alive")); err != nil {
		t.Fatalf("replacement socket write: %v", err)
	}
	_, payload, err := newClient.ReadMessage()
	if err != nil || string(payload) != "alive" {
		t.Fatalf("replacement socket read = %q, %v", payload, err)
	}
	_ = oldClient.Close()
	manager.Disconnect(EventCtx{From: "reused", Socket: newServer})
	if agent := receiveWithin(t, closeEvents); agent != "new-agent" {
		t.Fatalf("new close agent = %q, want new-agent", agent)
	}
	receiveWithin(t, newWorker.done)
	if _, ok := manager.clients.Get("reused"); ok {
		t.Fatal("new disconnect left connection indexed")
	}
	if manager.eventDispatcher.currentClient("reused") != nil {
		t.Fatal("new disconnect left worker indexed")
	}
}

func TestClientEventFIFOCloseContextPreservesHandshakeMetadata(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	closed := make(chan *FromCtx, 1)
	manager.On(Event(EventClose).String(), func(ctx EventCtx) bool {
		closed <- ctx.FromCtx
		return true
	})
	initial := &FromCtx{
		UserAgent:  "agent",
		RequestURI: "/socket?room=1",
		Addr:       AddrCtx{RemoteAddr: "remote", XRealIP: "real", XForwardedFor: "forwarded"},
	}
	worker := manager.registerConnectionSync(EventCtx{From: "client", FromCtx: initial})
	manager.BindUid("client", "uid")
	manager.JoinGroup("client", "room")
	manager.unregisterConnectionSync(EventCtx{From: "client"}, worker)
	ctx := receiveWithin(t, closed)
	if ctx.UserAgent != initial.UserAgent || ctx.RequestURI != initial.RequestURI || ctx.Addr != initial.Addr {
		t.Fatalf("handshake metadata lost: %#v", ctx)
	}
	if ctx.Uid != "uid" || !reflect.DeepEqual(ctx.Groups, []string{"room"}) {
		t.Fatalf("dynamic close context = %#v", ctx)
	}
}

func TestEventDispatcherRejectsMissingFromWithoutCreatingGlobalWorker(t *testing.T) {
	manager := NewWebsocketManager()
	if !manager.dispatchUserEvent("missing", EventCtx{}) {
		t.Fatal("missing handler should be a no-op")
	}
	manager.On("invalid-global", func(EventCtx) bool { return true })
	if manager.dispatchUserEvent("invalid-global", EventCtx{}) {
		t.Fatal("non-legacy event without From should be rejected")
	}
	manager.eventDispatcher.mu.RLock()
	global := manager.eventDispatcher.global
	manager.eventDispatcher.mu.RUnlock()
	if global != nil {
		t.Fatal("missing global handler created a worker")
	}
}

func TestClientEventFIFOConnectThenDisconnectUsesManagerOrderAndAsyncReturn(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{EventQueueSize: 4})
	registerEntered := make(chan struct{})
	releaseRegister := make(chan struct{})
	unregisterEntered := make(chan struct{})
	releaseUnregister := make(chan struct{})
	manager.beforeRegisterConnection = func() {
		close(registerEntered)
		<-releaseRegister
	}
	manager.beforeUnregisterConnection = func() {
		close(unregisterEntered)
		<-releaseUnregister
	}

	connectReturned := make(chan struct{})
	go func() {
		manager.Connect(EventCtx{From: "ordered"})
		close(connectReturned)
	}()
	receiveWithin(t, registerEntered)
	receiveWithin(t, connectReturned)

	disconnectReturned := make(chan struct{})
	go func() {
		manager.Disconnect(EventCtx{From: "ordered"})
		close(disconnectReturned)
	}()
	close(releaseRegister)
	receiveWithin(t, unregisterEntered)
	receiveWithin(t, disconnectReturned)
	if manager.ClientCount() != 1 {
		t.Fatalf("disconnect waited for processing; client count = %d, want 1 while hook blocks", manager.ClientCount())
	}
	close(releaseUnregister)
	waitForCondition(t, func() bool { return manager.ClientCount() == 0 })
}

func receiveWithin[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for channel")
		var zero T
		return zero
	}
}

func waitForCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !condition() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for condition")
		default:
			runtime.Gosched()
		}
	}
}
