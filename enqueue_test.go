package goWebsocket

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
)

type countingMarshalPayload struct {
	calls *atomic.Int32
	value string
}

func (p countingMarshalPayload) MarshalJSON() ([]byte, error) {
	p.calls.Add(1)
	return json.Marshal(map[string]string{"value": p.value})
}

type failingMarshalPayload struct{}

func (failingMarshalPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("secret-marshal-error")
}

func dormantWriter(queueSize int) *connectionWriter {
	return &connectionWriter{
		outbound:      make(chan outboundFrame, queueSize),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		heartbeatDone: make(chan struct{}),
	}
}

func publishTestRecipient(manager *WebsocketManager, clientID string, writer *connectionWriter) *clientEventWorker {
	worker := &clientEventWorker{}
	manager.connectionRecords.Set(clientID, connectionRecord{worker: worker, writer: writer, ready: true})
	manager.clients.Set(clientID, ConnectionCtx{Group: make(map[string]bool)})
	return worker
}

func addTestGroup(manager *WebsocketManager, group string, clientIDs ...string) {
	members := cmap.New[bool]()
	for _, clientID := range clientIDs {
		members.Set(clientID, true)
	}
	manager.groups.Set(group, members)
}

func addTestUser(manager *WebsocketManager, uid string, clientIDs ...string) {
	members := cmap.New[bool]()
	for _, clientID := range clientIDs {
		members.Set(clientID, true)
	}
	manager.users.Set(uid, members)
}

func TestEnqueueToGroupReturnsStableSortedNonNilResultsAndMarshalsOnce(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 4})
	aWriter := dormantWriter(4)
	bWriter := dormantWriter(4)
	publishTestRecipient(manager, "b", bWriter)
	publishTestRecipient(manager, "a", aWriter)
	addTestGroup(manager, "room", "b", "a")

	var calls atomic.Int32
	result := manager.EnqueueToGroup("room", countingMarshalPayload{calls: &calls, value: "same"})
	if calls.Load() != 1 {
		t.Fatalf("marshal calls = %d, want 1", calls.Load())
	}
	if !reflect.DeepEqual(result.Accepted, []string{"a", "b"}) {
		t.Fatalf("accepted = %v", result.Accepted)
	}
	if result.Rejected == nil || len(result.Rejected) != 0 {
		t.Fatalf("rejected = %#v, want non-nil empty", result.Rejected)
	}
	aFrame := <-aWriter.outbound
	bFrame := <-bWriter.outbound
	if !bytes.Equal(aFrame.payload, bFrame.payload) || string(aFrame.payload) != `{"value":"same"}` {
		t.Fatalf("payloads differ: %q / %q", aFrame.payload, bFrame.payload)
	}

	result.Accepted[0] = "mutated"
	next := manager.EnqueueToGroup("room", map[string]string{"value": "next"})
	if !reflect.DeepEqual(next.Accepted, []string{"a", "b"}) {
		t.Fatalf("subsequent accepted was aliased: %v", next.Accepted)
	}

	empty := manager.EnqueueToGroup("missing", nil)
	if empty.Accepted == nil || empty.Rejected == nil || len(empty.Accepted) != 0 || len(empty.Rejected) != 0 {
		t.Fatalf("empty result = %#v", empty)
	}
}

func TestEnqueueToGroupRejectsFullAndContinuesOtherSnapshotMembers(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 1})
	fullWriter := dormantWriter(1)
	fullWriter.outbound <- outboundFrame{messageType: websocket.TextMessage, payload: []byte("occupied")}
	acceptedWriter := dormantWriter(1)
	publishTestRecipient(manager, "a-full", fullWriter)
	publishTestRecipient(manager, "b-open", acceptedWriter)
	addTestGroup(manager, "room", "b-open", "a-full")

	result := manager.EnqueueToGroup("room", map[string]bool{"ok": true})
	if !reflect.DeepEqual(result.Accepted, []string{"b-open"}) || !reflect.DeepEqual(result.Rejected, []string{"a-full"}) {
		t.Fatalf("result = %#v", result)
	}
	select {
	case <-acceptedWriter.outbound:
	case <-time.After(3 * time.Second):
		t.Fatal("later snapshot member was not enqueued after prior queue-full")
	}
}

func TestEnqueueToGroupRejectsClosedOrReplacedSnapshotIdentity(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2})
	oldWriter := dormantWriter(2)
	oldWorker := publishTestRecipient(manager, "client", oldWriter)
	addTestGroup(manager, "room", "client")

	snapshot := manager.snapshotGroupRecipients("room")
	newWriter := dormantWriter(2)
	newWorker := &clientEventWorker{}
	manager.connectionRecords.Set("client", connectionRecord{worker: newWorker, writer: newWriter, ready: true})
	manager.clients.Set("client", ConnectionCtx{Group: make(map[string]bool)})
	result := manager.enqueueRecipients(snapshot, websocket.TextMessage, []byte("old"))
	if !reflect.DeepEqual(result.Rejected, []string{"client"}) || len(result.Accepted) != 0 {
		t.Fatalf("replacement result = %#v", result)
	}
	if len(newWriter.outbound) != 0 || len(oldWriter.outbound) != 0 {
		t.Fatal("snapshot payload leaked to old or replacement writer")
	}
	if oldWorker == newWorker {
		t.Fatal("test did not replace identity")
	}

	snapshot = manager.snapshotGroupRecipients("room")
	manager.clients.Remove("client")
	manager.connectionRecords.Remove("client")
	result = manager.enqueueRecipients(snapshot, websocket.TextMessage, []byte("closed"))
	if !reflect.DeepEqual(result.Rejected, []string{"client"}) || len(result.Accepted) != 0 {
		t.Fatalf("closed result = %#v", result)
	}
}

func TestEnqueueToGroupMarshalFailureIsEmptyAndDoesNotLeakDataOrError(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	manager := NewWebsocketManagerWithConfig(ManagerConfig{Debug: true})
	result := manager.EnqueueToGroup("secret-room", failingMarshalPayload{})
	if result.Accepted == nil || result.Rejected == nil || len(result.Accepted) != 0 || len(result.Rejected) != 0 {
		t.Fatalf("marshal failure result = %#v", result)
	}
	logged := output.String()
	if !strings.Contains(logged, "[WebsocketMarshalError]") {
		t.Fatalf("missing fixed classification log: %q", logged)
	}
	for _, secret := range []string{"secret-room", "secret-marshal-error"} {
		if strings.Contains(logged, secret) {
			t.Fatalf("log leaked %q: %q", secret, logged)
		}
	}
}

func TestLegacySendAPIsEnqueueAndDispatchExactlyOnceWithCompatibleContexts(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 8, EventQueueSize: 8})
	writer := dormantWriter(8)
	publishTestRecipient(manager, "client", writer)
	addTestGroup(manager, "room", "client")
	addTestUser(manager, "user", "client")

	type callback struct {
		event string
		ctx   EventCtx
	}
	callbacks := make(chan callback, 8)
	register := func(event string) {
		manager.On(event, func(ctx EventCtx) bool {
			callbacks <- callback{event: event, ctx: ctx}
			return true
		})
	}
	for _, event := range []string{
		Event(EventSendToClient).String(),
		Event(EventSendToGroup).String(),
		Event(EventSendToUid).String(),
		Event(EventBroadcast).String(),
	} {
		register(event)
	}

	manager.Send("client", "direct")
	manager.SendToGroup("room", "group")
	manager.SendToUid("user", "uid")
	manager.SendToAll("all")

	seen := make(map[string]EventCtx)
	for i := 0; i < 4; i++ {
		select {
		case got := <-callbacks:
			if _, exists := seen[got.event]; exists {
				t.Fatalf("duplicate callback for %s", got.event)
			}
			seen[got.event] = got.ctx
		case <-time.After(3 * time.Second):
			t.Fatalf("callbacks received = %d, want 4", len(seen))
		}
	}
	if len(writer.outbound) != 4 {
		t.Fatalf("queued frames = %d, want 4", len(writer.outbound))
	}
	if got := seen[Event(EventSendToClient).String()]; got.ToId != "client" || got.Data != "direct" {
		t.Fatalf("send context = %#v", got)
	}
	if got := seen[Event(EventSendToGroup).String()]; got.ToGroup != "room" || got.Data != "group" {
		t.Fatalf("group context = %#v", got)
	}
	if got := seen[Event(EventSendToUid).String()]; got.ToUid != "user" || got.Data != "uid" {
		t.Fatalf("uid context = %#v", got)
	}
	if got := seen[Event(EventBroadcast).String()]; got.Data != "all" {
		t.Fatalf("broadcast context = %#v", got)
	}
	select {
	case duplicate := <-callbacks:
		t.Fatalf("unexpected duplicate callback: %#v", duplicate)
	default:
	}
}

func TestLegacySendToGroupMarshalsPayloadOnce(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2})
	aWriter := dormantWriter(2)
	bWriter := dormantWriter(2)
	publishTestRecipient(manager, "a", aWriter)
	publishTestRecipient(manager, "b", bWriter)
	addTestGroup(manager, "room", "a", "b")
	var calls atomic.Int32

	manager.SendToGroup("room", countingMarshalPayload{calls: &calls, value: "legacy"})
	if calls.Load() != 1 {
		t.Fatalf("legacy group marshal calls = %d, want 1", calls.Load())
	}
	if len(aWriter.outbound) != 1 || len(bWriter.outbound) != 1 {
		t.Fatalf("legacy group queue lengths = %d/%d", len(aWriter.outbound), len(bWriter.outbound))
	}
}

func TestLegacySendCallbacksAreAsyncReentrantAndPanicIsolatedWithSmallEventQueue(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 8, EventQueueSize: 1})
	writer := dormantWriter(8)
	publishTestRecipient(manager, "client", writer)
	addTestGroup(manager, "room", "client")
	addTestUser(manager, "user", "client")

	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })
	manager.Config.Debug = true

	callbacks := make(chan string, 8)
	callbacksFinished := make(chan struct{}, 4)
	manager.afterLegacyUserEvent = func() { callbacksFinished <- struct{}{} }
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	directEvent := Event(EventSendToClient).String()
	manager.On(directEvent, func(ctx EventCtx) bool {
		close(firstStarted)
		<-releaseFirst
		manager.SendToAll("nested")
		if ctx.ToId == "client" && ctx.Data == "direct" {
			callbacks <- directEvent
		}
		return true
	})
	groupEvent := Event(EventSendToGroup).String()
	manager.On(groupEvent, func(ctx EventCtx) bool {
		if ctx.ToGroup == "room" && ctx.Data == "group" {
			callbacks <- groupEvent
		}
		return true
	})
	uidEvent := Event(EventSendToUid).String()
	manager.On(uidEvent, func(ctx EventCtx) bool {
		if ctx.ToUid == "user" && ctx.Data == "uid" {
			callbacks <- uidEvent
		}
		panic("secret-legacy-panic")
	})
	broadcastEvent := Event(EventBroadcast).String()
	manager.On(broadcastEvent, func(ctx EventCtx) bool {
		if ctx.Data == "nested" {
			callbacks <- broadcastEvent
		}
		return true
	})

	manager.Send("client", "direct")
	receiveWithin(t, firstStarted)
	groupReturned := make(chan struct{})
	go func() {
		manager.SendToGroup("room", "group")
		close(groupReturned)
	}()
	uidReturned := make(chan struct{})
	go func() {
		manager.SendToUid("user", "uid")
		close(uidReturned)
	}()
	waitForOutboundFrames(t, writer, 3)
	close(releaseFirst)
	receiveWithin(t, groupReturned)
	receiveWithin(t, uidReturned)
	waitForOutboundFrames(t, writer, 4)

	seen := make(map[string]int, 4)
	for i := 0; i < 4; i++ {
		seen[receiveWithin(t, callbacks)]++
	}
	for i := 0; i < 4; i++ {
		receiveWithin(t, callbacksFinished)
	}
	for _, event := range []string{directEvent, groupEvent, uidEvent, broadcastEvent} {
		if seen[event] != 1 {
			t.Fatalf("callback count for %s = %d, want 1", event, seen[event])
		}
	}
	select {
	case event := <-callbacks:
		t.Fatalf("unexpected duplicate callback: %s", event)
	default:
	}
	logged := output.String()
	if !strings.Contains(logged, "[WebsocketEventHandlerPanic]") {
		t.Fatalf("missing panic classification log: %q", logged)
	}
	if strings.Contains(logged, "secret-legacy-panic") {
		t.Fatalf("panic log leaked recovered value: %q", logged)
	}
}

func waitForOutboundFrames(t *testing.T, writer *connectionWriter, count int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for len(writer.outbound) < count {
		if time.Now().After(deadline) {
			t.Fatalf("outbound frames = %d, want %d", len(writer.outbound), count)
		}
		runtime.Gosched()
	}
}

func TestInboundSendToGroupCallbackIsNotDuplicatedByEnqueueAPI(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 2, EventQueueSize: 2})
	writer := dormantWriter(2)
	publishTestRecipient(manager, "target", writer)
	addTestGroup(manager, "room", "target")
	source := manager.eventDispatcher.replaceClient("source", EventCtx{From: "source"})
	called := make(chan struct{}, 2)
	marker := make(chan struct{})
	event := Event(EventSendToGroup).String()
	manager.On(event, func(ctx EventCtx) bool {
		manager.EnqueueToGroup(ctx.ToGroup, ctx.Data)
		called <- struct{}{}
		return true
	})
	manager.On("inbound-marker", func(EventCtx) bool {
		close(marker)
		return true
	})

	ctx := EventCtx{From: "source", ToGroup: "room", Data: "payload", Event: event}
	if !manager.dispatchConnectionUserEvent(source, event, ctx, false) {
		t.Fatal("inbound event submission failed")
	}
	if !manager.dispatchConnectionUserEvent(source, "inbound-marker", EventCtx{From: "source"}, false) {
		t.Fatal("inbound marker submission failed")
	}
	receiveWithin(t, marker)
	receiveWithin(t, called)
	select {
	case <-called:
		t.Fatal("inbound callback executed more than once")
	default:
	}
	if len(writer.outbound) != 1 {
		t.Fatalf("queued frames = %d, want 1", len(writer.outbound))
	}
}
