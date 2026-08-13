package goWebsocket

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingMessageWriter struct {
	started    chan struct{}
	release    <-chan struct{}
	startOnce  sync.Once
	frames     chan outboundFrame
	fail       atomic.Bool
	active     atomic.Int32
	concurrent atomic.Bool
}

func newRecordingMessageWriter(buffer int) *recordingMessageWriter {
	return &recordingMessageWriter{started: make(chan struct{}), frames: make(chan outboundFrame, buffer)}
}

func (w *recordingMessageWriter) SetWriteDeadline(time.Time) error { return nil }

func (w *recordingMessageWriter) WriteMessage(messageType int, payload []byte) error {
	if w.active.Add(1) != 1 {
		w.concurrent.Store(true)
	}
	defer w.active.Add(-1)
	w.startOnce.Do(func() { close(w.started) })
	if w.release != nil {
		<-w.release
	}
	if w.fail.Load() {
		return errors.New("sensitive transport error")
	}
	w.frames <- outboundFrame{messageType: messageType, payload: append([]byte(nil), payload...)}
	return nil
}

func TestConnectionWriterPreservesTextAndPingFIFOWithOneWriter(t *testing.T) {
	socket := newRecordingMessageWriter(3)
	writer := newConnectionWriter(socket, 4, 0, func(writerFailure) {})

	payload := []byte("a")
	if !writer.enqueue(websocket.TextMessage, payload) {
		t.Fatal("enqueue text a failed")
	}
	payload[0] = 'x'
	if !writer.enqueue(websocket.PingMessage, nil) {
		t.Fatal("enqueue ping failed")
	}
	if !writer.enqueue(websocket.TextMessage, []byte("b")) {
		t.Fatal("enqueue text b failed")
	}

	got := []outboundFrame{receiveWithin(t, socket.frames), receiveWithin(t, socket.frames), receiveWithin(t, socket.frames)}
	want := []outboundFrame{
		{messageType: websocket.TextMessage, payload: []byte("a")},
		{messageType: websocket.PingMessage, payload: nil},
		{messageType: websocket.TextMessage, payload: []byte("b")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("frames = %#v, want %#v", got, want)
	}
	if socket.concurrent.Load() {
		t.Fatal("socket had concurrent writers")
	}
	writer.stop()
	receiveWithin(t, writer.done)
}

func TestOutboundQueueFullIsFastAndDoesNotBlockAnotherWriter(t *testing.T) {
	releaseA := make(chan struct{})
	socketA := newRecordingMessageWriter(2)
	socketA.release = releaseA
	failureA := make(chan writerFailure, 1)
	writerA := newConnectionWriter(socketA, 1, 0, func(reason writerFailure) { failureA <- reason })
	socketB := newRecordingMessageWriter(1)
	writerB := newConnectionWriter(socketB, 1, 0, func(writerFailure) {})

	if !writerA.enqueue(websocket.TextMessage, []byte("writing")) {
		t.Fatal("enqueue active A frame failed")
	}
	receiveWithin(t, socketA.started)
	if !writerA.enqueue(websocket.TextMessage, []byte("queued")) {
		t.Fatal("enqueue queued A frame failed")
	}
	returned := make(chan bool, 1)
	go func() { returned <- writerA.enqueue(websocket.TextMessage, []byte("overflow")) }()
	if receiveWithin(t, returned) {
		t.Fatal("full A queue accepted frame")
	}
	if reason := receiveWithin(t, failureA); reason != writerFailureQueueFull {
		t.Fatalf("failure = %v, want queue full", reason)
	}

	if !writerB.enqueue(websocket.TextMessage, []byte("b")) {
		t.Fatal("slow A blocked enqueue to B")
	}
	if frame := receiveWithin(t, socketB.frames); string(frame.payload) != "b" {
		t.Fatalf("B payload = %q", frame.payload)
	}
	close(releaseA)
	receiveWithin(t, writerA.done)
	writerB.stop()
	receiveWithin(t, writerB.done)
}

func TestOutboundQueueFullAtomicallyRejectsConcurrentAndLaterEnqueue(t *testing.T) {
	releaseWrite := make(chan struct{})
	socket := newRecordingMessageWriter(2)
	socket.release = releaseWrite
	overflowObserved := make(chan struct{})
	releaseOverflow := make(chan struct{})
	failureCalled := make(chan struct{})
	writer := newConnectionWriter(socket, 1, 0, func(reason writerFailure) {
		if reason != writerFailureQueueFull {
			t.Errorf("failure = %v, want queue full", reason)
		}
		close(failureCalled)
	})
	writer.afterQueueFullObserved = func() {
		close(overflowObserved)
		<-releaseOverflow
	}
	if !writer.enqueue(websocket.TextMessage, []byte("writing")) {
		t.Fatal("enqueue active frame failed")
	}
	receiveWithin(t, socket.started)
	if !writer.enqueue(websocket.TextMessage, []byte("queued")) {
		t.Fatal("enqueue queued frame failed")
	}
	overflowReturned := make(chan bool, 1)
	go func() {
		overflowReturned <- writer.enqueue(websocket.TextMessage, []byte("overflow"))
	}()
	receiveWithin(t, overflowObserved)
	close(releaseWrite)
	receiveWithin(t, socket.frames)

	concurrentReturned := make(chan bool, 1)
	go func() {
		concurrentReturned <- writer.enqueue(websocket.TextMessage, []byte("concurrent"))
	}()
	if receiveWithin(t, concurrentReturned) {
		t.Fatal("concurrent enqueue succeeded after queue-full observation")
	}
	if writer.enqueue(websocket.TextMessage, []byte("later")) {
		t.Fatal("later enqueue succeeded after queue-full observation")
	}
	close(releaseOverflow)
	if receiveWithin(t, overflowReturned) {
		t.Fatal("overflow enqueue returned true")
	}
	receiveWithin(t, failureCalled)
	receiveWithin(t, writer.done)
}

func TestConnectionPublicationHidesWriterUntilPublicAndPreservesQueuedSends(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 4})
	publicServer, publicClient := websocketTestPair(t)
	pendingServer, pendingClient := websocketTestPair(t)
	publicWorker := manager.registerConnectionSync(EventCtx{From: "public", Socket: publicServer})

	recordReady := make(chan struct{})
	releasePublish := make(chan struct{})
	manager.beforePublishConnection = func(clientID string) {
		if clientID != "pending" {
			return
		}
		close(recordReady)
		<-releasePublish
	}
	pendingRegistered := make(chan *clientEventWorker, 1)
	go func() {
		pendingRegistered <- manager.registerConnectionSync(EventCtx{From: "pending", Socket: pendingServer})
	}()
	receiveWithin(t, recordReady)

	if record, exists := manager.connectionRecords.Get("pending"); !exists || !record.ready || record.writer == nil {
		t.Fatal("complete private record was not ready before public publication")
	}
	if _, exists := manager.clients.Get("pending"); exists {
		t.Fatal("pending connection became public before publication barrier")
	}
	if manager.enqueueClient("pending", nil, websocket.TextMessage, []byte("leak")) {
		t.Fatal("private record accepted data before public publication")
	}
	if !manager.enqueueClient("public", publicWorker, websocket.TextMessage, []byte("direct")) {
		t.Fatal("existing public connection became unavailable during pending publication")
	}
	_, direct, err := publicClient.ReadMessage()
	if err != nil || string(direct) != "direct" {
		t.Fatalf("public direct read = %q, %v", direct, err)
	}

	broadcastReturned := make(chan struct{})
	go func() {
		manager.SendToAll(map[string]string{"kind": "broadcast"})
		close(broadcastReturned)
	}()
	directReturned := make(chan struct{})
	go func() {
		manager.Send("pending", map[string]string{"kind": "direct"})
		close(directReturned)
	}()
	close(releasePublish)
	pendingWorker := receiveWithin(t, pendingRegistered)
	receiveWithin(t, broadcastReturned)
	receiveWithin(t, directReturned)

	readKinds := func(client *websocket.Conn, count int) map[string]int {
		t.Helper()
		got := make(map[string]int)
		for i := 0; i < count; i++ {
			var payload map[string]string
			if err := client.ReadJSON(&payload); err != nil {
				t.Fatalf("read json: %v", err)
			}
			got[payload["kind"]]++
		}
		return got
	}
	if got := readKinds(publicClient, 1); got["broadcast"] != 1 {
		t.Fatalf("public messages = %v", got)
	}
	if got := readKinds(pendingClient, 2); got["broadcast"] != 1 || got["direct"] != 1 {
		t.Fatalf("pending messages = %v", got)
	}
	manager.unregisterConnectionSync(EventCtx{From: "public"}, publicWorker)
	manager.unregisterConnectionSync(EventCtx{From: "pending"}, pendingWorker)
}

func TestConnectionWriterWriteFailureAndRepeatedStopAreIdempotent(t *testing.T) {
	socket := newRecordingMessageWriter(1)
	socket.fail.Store(true)
	failed := make(chan writerFailure, 2)
	writer := newConnectionWriter(socket, 1, 0, func(reason writerFailure) { failed <- reason })
	if !writer.enqueue(websocket.TextMessage, []byte("x")) {
		t.Fatal("initial enqueue failed")
	}
	if reason := receiveWithin(t, failed); reason != writerFailureWrite {
		t.Fatalf("failure = %v, want write failure", reason)
	}
	receiveWithin(t, writer.done)
	writer.stop()
	writer.stop()
	if writer.enqueue(websocket.TextMessage, []byte("late")) {
		t.Fatal("stopped writer accepted a frame")
	}
	select {
	case reason := <-failed:
		t.Fatalf("duplicate failure callback: %v", reason)
	default:
	}
}

func TestConnectionWriterStopDropsQueuedFramesAndClosesDone(t *testing.T) {
	release := make(chan struct{})
	socket := newRecordingMessageWriter(4)
	socket.release = release
	writer := newConnectionWriter(socket, 4, time.Millisecond, func(reason writerFailure) {
		t.Errorf("unexpected failure after stop: %v", reason)
	})
	if !writer.enqueue(websocket.TextMessage, []byte("active")) {
		t.Fatal("enqueue active frame failed")
	}
	receiveWithin(t, socket.started)
	if !writer.enqueue(websocket.TextMessage, []byte("queued-a")) ||
		!writer.enqueue(websocket.TextMessage, []byte("queued-b")) {
		t.Fatal("enqueue queued frames failed")
	}
	writer.stop()
	close(release)
	receiveWithin(t, writer.done)
	if first := receiveWithin(t, socket.frames); string(first.payload) != "active" {
		t.Fatalf("active payload = %q", first.payload)
	}
	select {
	case frame := <-socket.frames:
		t.Fatalf("stop drained queued frame: %#v", frame)
	default:
	}
}

func TestConnectionWriterOldFailureCannotRemoveReplacement(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2})
	oldServer, _ := websocketTestPair(t)
	newServer, newClient := websocketTestPair(t)
	oldWorker := manager.registerConnectionSync(EventCtx{From: "reused", Socket: oldServer})
	oldRecord, ok := manager.connectionRecords.Get("reused")
	if !ok || oldRecord.writer == nil {
		t.Fatal("old writer was not published")
	}
	newWorker := manager.registerConnectionSync(EventCtx{From: "reused", Socket: newServer})
	newRecord, ok := manager.connectionRecords.Get("reused")
	if !ok || newRecord.writer == nil || newWorker == oldWorker {
		t.Fatal("replacement writer was not published")
	}
	receiveWithin(t, oldRecord.writer.done)

	manager.handleWriterFailure("reused", oldWorker, writerFailureWrite)
	waitForCondition(t, func() bool { return manager.currentClientWorker("reused") == newWorker })
	if !newRecord.writer.enqueue(websocket.TextMessage, []byte("alive")) {
		t.Fatal("old failure stopped replacement writer")
	}
	_, payload, err := newClient.ReadMessage()
	if err != nil || string(payload) != "alive" {
		t.Fatalf("replacement read = %q, %v", payload, err)
	}
	manager.unregisterConnectionSync(EventCtx{From: "reused"}, newWorker)
}

func TestOutboundQueueManagerLoopDoesNotWaitForSocketWrite(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2})
	releaseA := make(chan struct{})
	socketA := newRecordingMessageWriter(1)
	socketA.release = releaseA
	socketB := newRecordingMessageWriter(1)
	workerA := manager.eventDispatcher.replaceClient("a")
	workerB := manager.eventDispatcher.replaceClient("b")
	writerA := newConnectionWriter(socketA, 2, 0, func(writerFailure) {})
	writerB := newConnectionWriter(socketB, 2, 0, func(writerFailure) {})
	manager.clients.Set("a", ConnectionCtx{Group: make(map[string]bool)})
	manager.clients.Set("b", ConnectionCtx{Group: make(map[string]bool)})
	manager.connectionRecords.Set("a", connectionRecord{worker: workerA, writer: writerA, ready: true})
	manager.connectionRecords.Set("b", connectionRecord{worker: workerB, writer: writerB, ready: true})

	manager.Send("a", map[string]string{"value": "slow"})
	receiveWithin(t, socketA.started)
	manager.Send("b", map[string]string{"value": "fast"})
	frame := receiveWithin(t, socketB.frames)
	if string(frame.payload) != `{"value":"fast"}` {
		t.Fatalf("B payload = %s", frame.payload)
	}
	close(releaseA)
	writerA.stop()
	writerB.stop()
	receiveWithin(t, writerA.done)
	receiveWithin(t, writerB.done)
	workerA.submit(userEventTask{terminal: true})
	workerB.submit(userEventTask{terminal: true})
	receiveWithin(t, workerA.done)
	receiveWithin(t, workerB.done)
}

func TestConnectionWriterRealWebSocketTextAndPing(t *testing.T) {
	serverSide, clientSide := websocketTestPair(t)
	writer := newConnectionWriter(serverSide, 4, 0, func(writerFailure) {})
	if !writer.enqueue(websocket.TextMessage, []byte("a")) ||
		!writer.enqueue(websocket.PingMessage, nil) ||
		!writer.enqueue(websocket.TextMessage, []byte("b")) {
		t.Fatal("enqueue real websocket frames failed")
	}
	_, first, err := clientSide.ReadMessage()
	if err != nil || string(first) != "a" {
		t.Fatalf("first read = %q, %v", first, err)
	}
	_, second, err := clientSide.ReadMessage()
	if err != nil || string(second) != "b" {
		t.Fatalf("second read = %q, %v", second, err)
	}
	writer.stop()
	receiveWithin(t, writer.done)
}

func TestConnectionWriterReadEOFStopsWriterAndRemovesInstance(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2})
	connected := make(chan string, 1)
	manager.On(Event(EventConnect).String(), func(ctx EventCtx) bool {
		connected <- ctx.From
		return true
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manager.Handler(w, r, nil)
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	clientID := receiveWithin(t, connected)
	record, ok := manager.connectionRecords.Get(clientID)
	if !ok || record.writer == nil {
		t.Fatal("connection writer missing")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	receiveWithin(t, record.writer.done)
	waitForCondition(t, func() bool {
		_, exists := manager.connectionRecords.Get(clientID)
		return !exists
	})
}
