package goWebsocket

import (
	"bytes"
	"log"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycleUpgradeFailureLogUsesFixedCategory(t *testing.T) {
	manager := NewWebsocketManager(true)
	defer manager.Close()
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	request := httptest.NewRequest("GET", "http://example.test/private?token=secret", nil)
	recorder := httptest.NewRecorder()
	manager.Handler(recorder, request, nil)
	logged := output.String()
	if !strings.Contains(logged, "[WebsocketUpgradeError]") {
		t.Fatalf("missing fixed upgrade category: %q", logged)
	}
	if strings.Contains(logged, "client is not using") || strings.Contains(logged, "secret") {
		t.Fatalf("upgrade log contains dynamic details: %q", logged)
	}
}

func TestManagerCloseBeforeConnectionsIsConcurrentAndIdempotent(t *testing.T) {
	manager := NewWebsocketManager()

	const callers = 16
	errors := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		go func() {
			ready.Done()
			<-start
			errors <- manager.Close()
		}()
	}
	ready.Wait()
	close(start)
	for i := 0; i < callers; i++ {
		select {
		case err := <-errors:
			if err != nil {
				t.Fatalf("Close returned error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent Close blocked")
		}
	}
	select {
	case <-manager.registerDone:
	default:
		t.Fatal("Close returned before register loop stopped")
	}
}

func TestManagerCloseRejectsPublicMutationsWithoutBlocking(t *testing.T) {
	manager := NewWebsocketManager()
	var callbacks atomic.Int32
	for _, eventName := range []string{
		Event(EventSendToClient).String(),
		Event(EventSendToGroup).String(),
		Event(EventSendToUid).String(),
		Event(EventBroadcast).String(),
	} {
		manager.On(eventName, func(EventCtx) bool {
			callbacks.Add(1)
			return true
		})
	}
	if err := manager.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	returned := make(chan struct{})
	go func() {
		manager.Connect(EventCtx{From: "closed-client"})
		manager.Disconnect(EventCtx{From: "closed-client"})
		manager.BindUid("closed-client", "uid")
		manager.UnbindUid("closed-client", "uid")
		manager.JoinGroup("closed-client", "group")
		manager.LeaveGroup("closed-client", "group")
		manager.Send("closed-client", "secret")
		manager.SendToGroup("group", "secret")
		manager.SendToUid("uid", "secret")
		manager.SendToAll("secret")
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(3 * time.Second):
		t.Fatal("public mutation blocked after Close")
	}

	result := manager.EnqueueToGroup("group", "secret")
	if len(result.Accepted) != 0 || len(result.Rejected) != 0 {
		t.Fatalf("closed EnqueueToGroup result = %#v, want empty", result)
	}
	if callbacks.Load() != 0 {
		t.Fatalf("legacy callbacks after Close = %d, want 0", callbacks.Load())
	}
	if manager.On("late", func(EventCtx) bool { return true }) {
		t.Fatal("On accepted a handler after Close")
	}
}

func TestManagerCloseCancelsPublicChannelSubmitWhileRegisterLoopIsBusy(t *testing.T) {
	manager := NewWebsocketManager()
	registerReceived := make(chan struct{})
	releaseRegister := make(chan struct{})
	manager.beforeRegisterConnection = func() {
		close(registerReceived)
		<-releaseRegister
	}
	firstReturned := make(chan struct{})
	go func() {
		manager.Connect(EventCtx{From: "first"})
		close(firstReturned)
	}()
	receiveWithin(t, registerReceived)
	receiveWithin(t, firstReturned)

	blockedSubmitReturned := make(chan struct{})
	go func() {
		manager.BindUid("second", "uid")
		close(blockedSubmitReturned)
	}()
	closeReturned := make(chan struct{})
	go func() {
		_ = manager.Close()
		close(closeReturned)
	}()

	receiveWithin(t, blockedSubmitReturned)
	close(releaseRegister)
	receiveWithin(t, closeReturned)
}

func TestManagerCloseWaitsForHandedOffLegacyRequestAckAndCallback(t *testing.T) {
	manager := NewWebsocketManagerWithConfig(ManagerConfig{OutboundQueueSize: 4})
	writer := dormantWriter(4)
	worker := manager.eventDispatcher.replaceClient("client")
	manager.connectionRecords.Set("client", connectionRecord{worker: worker, writer: writer, ready: true})
	manager.clients.Set("client", ConnectionCtx{Group: make(map[string]bool)})
	requestReceived := make(chan struct{})
	releaseAck := make(chan struct{})
	manager.beforeSendRequestAck = func() {
		close(requestReceived)
		<-releaseAck
	}
	callback := make(chan struct{}, 1)
	manager.On(Event(EventSendToClient).String(), func(EventCtx) bool {
		callback <- struct{}{}
		return true
	})

	sendReturned := make(chan struct{})
	go func() {
		manager.Send("client", "payload-secret")
		close(sendReturned)
	}()
	receiveWithin(t, requestReceived)
	closeReturned := make(chan struct{})
	go func() {
		_ = manager.Close()
		close(closeReturned)
	}()
	receiveWithin(t, manager.context.Done())
	if len(writer.outbound) != 1 {
		t.Fatalf("outbound frames before ack = %d, want 1", len(writer.outbound))
	}
	manager.clients.Remove("client")
	manager.connectionRecords.Remove("client")
	worker.submit(userEventTask{terminal: true})
	close(releaseAck)
	receiveWithin(t, sendReturned)
	receiveWithin(t, callback)
	receiveWithin(t, closeReturned)
	if len(writer.outbound) != 1 {
		t.Fatalf("outbound frames = %d, want 1", len(writer.outbound))
	}
	receiveWithin(t, worker.done)
	select {
	case <-callback:
		t.Fatal("legacy callback ran more than once")
	default:
	}
}

func TestManagerCloseCannotForceRunningUserHandlerButWorkerDrainsAfterRelease(t *testing.T) {
	manager := NewWebsocketManager()
	started := make(chan struct{})
	release := make(chan struct{})
	manager.On(Event(EventConnect).String(), func(EventCtx) bool {
		close(started)
		<-release
		return true
	})
	worker := manager.registerConnectionSync(EventCtx{From: "blocked"})
	receiveWithin(t, started)

	closed := make(chan struct{})
	go func() {
		_ = manager.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close waited for a user handler that cannot be forcibly interrupted")
	}
	close(release)
	receiveWithin(t, worker.done)
}

func TestManagerCloseHandlerCanReenterManagerAndClose(t *testing.T) {
	manager := NewWebsocketManager()
	closeHandlerDone := make(chan struct{})
	var closeCount atomic.Int32
	manager.On(Event(EventClose).String(), func(ctx EventCtx) bool {
		closeCount.Add(1)
		if ctx.FromCtx == nil || ctx.FromCtx.UserAgent != "tv-client" || ctx.FromCtx.Uid != "tv-user" {
			t.Errorf("Close context = %#v", ctx.FromCtx)
		}
		manager.Send(ctx.From, "ignored")
		manager.Disconnect(ctx)
		if err := manager.Close(); err != nil {
			t.Errorf("reentrant Close returned error: %v", err)
		}
		close(closeHandlerDone)
		return true
	})
	worker := manager.registerConnectionSync(EventCtx{
		From:    "reentrant",
		FromCtx: &FromCtx{UserAgent: "tv-client"},
	})
	worker.updateDynamicContext("tv-user", []string{"room"})

	if err := manager.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	receiveWithin(t, closeHandlerDone)
	receiveWithin(t, worker.done)
	if closeCount.Load() != 1 {
		t.Fatalf("Close handler count = %d, want 1", closeCount.Load())
	}
	if manager.ClientCount() != 0 {
		t.Fatalf("clients after Close = %d, want 0", manager.ClientCount())
	}
}
