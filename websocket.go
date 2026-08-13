package goWebsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	pongWait        = 60 * time.Second
	writeWait       = 45 * time.Second
	writeTicker     = 45 * time.Second
	pingMessageType = websocket.PingMessage

	readBufferSize  = 1024
	writeBufferSize = 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  readBufferSize,
	WriteBufferSize: writeBufferSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type H map[string]interface{}

type SeqOpCtx struct {
	ClientId string `json:"client_id,omitempty"`
	Uid      string `json:"uid,omitempty"`
	Group    string `json:"group,omitempty"`
}

type connectionRegistration struct {
	ctx EventCtx
	ack chan *clientEventWorker
}

type connectionUnregistration struct {
	ctx    EventCtx
	worker *clientEventWorker
	ack    chan struct{}
}

type sendRequest struct {
	ctx  EventCtx
	done chan struct{}
}

type connectionRecord struct {
	socket *websocket.Conn
	worker *clientEventWorker
	writer *connectionWriter
	ready  bool
}

type managerShutdownResources struct {
	writers []*connectionWriter
}

type WebsocketManager struct {
	Config       ManagerConfig
	context      context.Context
	cancel       context.CancelFunc
	closed       atomic.Bool
	closeOnce    sync.Once
	done         chan struct{}
	registerDone chan struct{}
	shutdown     chan chan managerShutdownResources
	lifecycleMu  sync.Mutex
	operationWG  sync.WaitGroup
	readWG       sync.WaitGroup

	eventHandlers     map[string]EventHandler
	userEventHandlers map[string]EventHandler
	handlerMu         sync.RWMutex
	eventDispatcher   *eventDispatcher

	clients           cmap.ConcurrentMap[string, ConnectionCtx] // [ClientId => ConnectionCtx]
	connectionRecords cmap.ConcurrentMap[string, connectionRecord]
	users             cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]] // [Uid => ClientMapEmpty]
	groups            cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]] // [GroupName => ClientMapEmpty]

	// events
	register       chan EventCtx
	registerSync   chan connectionRegistration
	unregisterSync chan connectionUnregistration
	unregister     chan connectionUnregistration
	bind           chan EventCtx
	unbind         chan EventCtx
	join           chan EventCtx
	leave          chan EventCtx
	send           chan EventCtx
	sendToGroup    chan EventCtx
	sendToUid      chan EventCtx
	broadcast      chan EventCtx
	sendSync       chan sendRequest
	sendGroupSync  chan sendRequest
	sendUidSync    chan sendRequest
	broadcastSync  chan sendRequest

	// 通过该锁控制连接相关的users、groups顺序增加/修改/删除等操作，防止并发导致操作不符合预期结果，同时会有性能问题
	sync.RWMutex

	beforeRegisterConnection   func()
	beforeUnregisterConnection func()
	beforePublishConnection    func(string)
	afterLegacyUserEvent       func()
	beforeSendRequestAck       func()
}

func (x *WebsocketManager) registerEvent() {
	defer close(x.registerDone)
	for {
		select {
		case response := <-x.shutdown:
			response <- x.shutdownConnections()
			return
		case ctx := <-x.register:
			if x.beforeRegisterConnection != nil {
				x.beforeRegisterConnection()
			}
			x.registerConnection(ctx)
		case request := <-x.registerSync:
			request.ack <- x.registerConnection(request.ctx)
		case request := <-x.unregisterSync:
			x.unregisterConnection(request.ctx, request.worker)
			close(request.ack)
		case request := <-x.unregister:
			if x.beforeUnregisterConnection != nil {
				x.beforeUnregisterConnection()
			}
			worker := x.resolveUnregisterWorker(request.ctx)
			if worker != nil {
				x.unregisterConnection(request.ctx, worker)
			}
			if request.ack != nil {
				close(request.ack)
			}
		case ctx := <-x.bind:
			x.bindUid(ctx.From, ctx.ToUid)
			x.refreshWorkerDynamicContext(ctx.From)
			if !x.dispatchUserEvent(Event(EventBindUid).String(), ctx) {
				x.disconnectEventQueueOverflowInLoop(ctx.From, x.currentClientWorker(ctx.From))
			}
		case ctx := <-x.unbind:
			x.unbindUid(ctx.From, ctx.ToUid)
			x.refreshWorkerDynamicContext(ctx.From)
			if !x.dispatchUserEvent(Event(EventUnbindUid).String(), ctx) {
				x.disconnectEventQueueOverflowInLoop(ctx.From, x.currentClientWorker(ctx.From))
			}
		case ctx := <-x.join:
			x.joinGroup(ctx.From, ctx.ToGroup)
			x.refreshWorkerDynamicContext(ctx.From)
			if !x.dispatchUserEvent(Event(EventJoinGroup).String(), ctx) {
				x.disconnectEventQueueOverflowInLoop(ctx.From, x.currentClientWorker(ctx.From))
			}
		case ctx := <-x.leave:
			x.leaveGroup(ctx.From, ctx.ToGroup)
			x.refreshWorkerDynamicContext(ctx.From)
			if !x.dispatchUserEvent(Event(EventLeaveGroup).String(), ctx) {
				x.disconnectEventQueueOverflowInLoop(ctx.From, x.currentClientWorker(ctx.From))
			}
		case ctx := <-x.send:
			x._send(ctx.ToId, websocket.TextMessage, ctx.Data)
		case ctx := <-x.sendToGroup:
			x.enqueueToGroup(ctx.ToGroup, ctx.Data)
		case ctx := <-x.sendToUid:
			if payload, ok := x.marshalOutbound(ctx.Data); ok {
				x.enqueueRecipients(x.snapshotUserRecipients(ctx.ToUid), websocket.TextMessage, payload)
			}
		case ctx := <-x.broadcast:
			if payload, ok := x.marshalOutbound(ctx.Data); ok {
				x.enqueueRecipients(x.snapshotAllRecipients(), websocket.TextMessage, payload)
			}
		case request := <-x.sendSync:
			x._send(request.ctx.ToId, websocket.TextMessage, request.ctx.Data)
			x.ackSendRequest(request)
		case request := <-x.sendGroupSync:
			x.enqueueToGroup(request.ctx.ToGroup, request.ctx.Data)
			x.ackSendRequest(request)
		case request := <-x.sendUidSync:
			if payload, ok := x.marshalOutbound(request.ctx.Data); ok {
				x.enqueueRecipients(x.snapshotUserRecipients(request.ctx.ToUid), websocket.TextMessage, payload)
			}
			x.ackSendRequest(request)
		case request := <-x.broadcastSync:
			if payload, ok := x.marshalOutbound(request.ctx.Data); ok {
				x.enqueueRecipients(x.snapshotAllRecipients(), websocket.TextMessage, payload)
			}
			x.ackSendRequest(request)
		}
	}
}

func (x *WebsocketManager) ackSendRequest(request sendRequest) {
	if x.beforeSendRequestAck != nil {
		x.beforeSendRequestAck()
	}
	close(request.done)
}

func (x *WebsocketManager) shutdownConnections() managerShutdownResources {
	resources := managerShutdownResources{}
	for clientID, record := range x.connectionRecords.Items() {
		if record.writer != nil {
			resources.writers = append(resources.writers, record.writer)
		}
		x.unregisterConnection(EventCtx{From: clientID}, record.worker)
	}
	for _, worker := range x.eventDispatcher.snapshotWorkers() {
		if worker.clientID == "" {
			worker.submit(userEventTask{terminal: true})
		}
	}
	return resources
}

// Close stops accepting manager mutations, closes active transports, and waits
// for manager-owned core loops, readers, writers, and heartbeats. It does not
// wait for connection FIFO, terminal, or legacy user handlers: Go cannot
// forcibly interrupt them, and terminal workers clean up by identity after
// their handlers return. See README for the complete lifecycle boundary.
func (x *WebsocketManager) Close() error {
	x.closeOnce.Do(func() {
		x.lifecycleMu.Lock()
		x.closed.Store(true)
		x.cancel()
		x.lifecycleMu.Unlock()
		go x.shutdownManager()
	})
	<-x.done
	return nil
}

func (x *WebsocketManager) shutdownManager() {
	x.operationWG.Wait()
	response := make(chan managerShutdownResources, 1)
	x.shutdown <- response
	resources := <-response
	<-x.registerDone
	for _, writer := range resources.writers {
		<-writer.done
	}
	x.readWG.Wait()
	close(x.done)
}

func (x *WebsocketManager) beginOperation() bool {
	x.lifecycleMu.Lock()
	defer x.lifecycleMu.Unlock()
	if x.closed.Load() {
		return false
	}
	x.operationWG.Add(1)
	return true
}

func (x *WebsocketManager) endOperation() {
	x.operationWG.Done()
}

func (x *WebsocketManager) dispatchUserEvent(eventName string, ctx EventCtx) bool {
	x.handlerMu.RLock()
	handler := x.userEventHandlers[eventName]
	x.handlerMu.RUnlock()
	if handler == nil {
		return true
	}
	if ctx.From == "" {
		return false
	}
	return x.submitConnectionUserEvent(x.currentClientWorker(ctx.From), handler, ctx, false)
}

func (x *WebsocketManager) dispatchLegacyUserEvent(eventName string, ctx EventCtx) {
	x.handlerMu.RLock()
	handler := x.userEventHandlers[eventName]
	x.handlerMu.RUnlock()
	if handler == nil {
		return
	}
	go func() {
		defer func() {
			if recover() != nil {
				x.Log("[WebsocketEventHandlerPanic]")
			}
			if x.afterLegacyUserEvent != nil {
				x.afterLegacyUserEvent()
			}
		}()
		handler(ctx)
	}()
}

func (x *WebsocketManager) dispatchConnectionUserEvent(worker *clientEventWorker, eventName string, ctx EventCtx, terminal bool) bool {
	x.handlerMu.RLock()
	handler := x.userEventHandlers[eventName]
	x.handlerMu.RUnlock()
	if handler == nil && !terminal {
		return true
	}
	return x.submitConnectionUserEvent(worker, handler, ctx, terminal)
}

func (x *WebsocketManager) submitConnectionUserEvent(worker *clientEventWorker, handler EventHandler, ctx EventCtx, terminal bool) bool {
	if worker == nil {
		return handler == nil && !terminal
	}
	return worker.submit(userEventTask{handler: handler, ctx: ctx, terminal: terminal})
}

func (x *WebsocketManager) registerConnection(ctx EventCtx) *clientEventWorker {
	if existing, ok := x.connectionRecords.Get(ctx.From); ok && existing.worker != nil {
		x.unregisterConnection(EventCtx{From: ctx.From}, existing.worker)
	}
	worker := x.eventDispatcher.replaceClient(ctx.From, ctx)
	x.connect(ctx, worker)
	if !x.dispatchConnectionUserEvent(worker, Event(EventConnect).String(), ctx, false) {
		x.disconnectEventQueueOverflowInLoop(ctx.From, worker)
	}
	return worker
}

func (x *WebsocketManager) registerConnectionSync(ctx EventCtx) *clientEventWorker {
	request := connectionRegistration{ctx: ctx, ack: make(chan *clientEventWorker, 1)}
	select {
	case <-x.context.Done():
		return nil
	case x.registerSync <- request:
	}
	select {
	case <-x.context.Done():
		return nil
	case worker := <-request.ack:
		return worker
	}
}

func (x *WebsocketManager) unregisterConnection(ctx EventCtx, worker *clientEventWorker) {
	if worker == nil {
		return
	}
	ctx.FromCtx = worker.snapshotFromCtx()
	if record, ok := x.connectionRecords.Get(ctx.From); ok && record.worker == worker && record.writer != nil {
		record.writer.stop()
	}
	if x.currentClientWorker(ctx.From) == worker {
		x.disconnectWorker(ctx.From, worker)
	}
	if worker.socket != nil {
		_ = worker.socket.Close()
	}
	x.dispatchConnectionUserEvent(worker, Event(EventClose).String(), ctx, true)
}

func (x *WebsocketManager) handleWriterFailure(clientID string, worker *clientEventWorker, reason writerFailure) {
	switch reason {
	case writerFailureQueueFull:
		x.Log("[WebsocketWriterQueueFull]")
	default:
		x.Log("[WebsocketWriterWriteError]")
	}
	go x.unregisterConnectionSync(EventCtx{From: clientID}, worker)
}

func (x *WebsocketManager) unregisterConnectionSync(ctx EventCtx, worker *clientEventWorker) {
	request := connectionUnregistration{ctx: ctx, worker: worker, ack: make(chan struct{})}
	select {
	case <-x.context.Done():
		return
	case <-x.registerDone:
		return
	case x.unregisterSync <- request:
	}
	select {
	case <-request.ack:
	case <-x.context.Done():
	case <-x.registerDone:
	}
}

func (x *WebsocketManager) currentClientWorker(clientID string) *clientEventWorker {
	record, ok := x.connectionRecords.Get(clientID)
	if !ok {
		return nil
	}
	return record.worker
}

func (x *WebsocketManager) resolveUnregisterWorker(ctx EventCtx) *clientEventWorker {
	record, ok := x.connectionRecords.Get(ctx.From)
	if !ok || record.worker == nil {
		return nil
	}
	if ctx.Socket != nil && record.socket != ctx.Socket {
		return nil
	}
	return record.worker
}

func (x *WebsocketManager) refreshWorkerDynamicContext(clientID string) {
	connection, ok := x.clients.Get(clientID)
	record, recordOK := x.connectionRecords.Get(clientID)
	if !ok || !recordOK || record.worker == nil {
		return
	}
	groups := make([]string, 0, len(connection.Group))
	for group := range connection.Group {
		groups = append(groups, group)
	}
	record.worker.updateDynamicContext(connection.Uid, groups)
}

func (x *WebsocketManager) disconnectEventQueueOverflow(clientID string, worker *clientEventWorker) {
	ctx := x.eventQueueOverflowContext(clientID, worker)
	if worker != nil && worker.socket != nil {
		_ = worker.socket.Close()
	}
	x.unregisterConnectionSync(ctx, worker)
}

func (x *WebsocketManager) disconnectEventQueueOverflowInLoop(clientID string, worker *clientEventWorker) {
	ctx := x.eventQueueOverflowContext(clientID, worker)
	if worker != nil && worker.socket != nil {
		_ = worker.socket.Close()
	}
	x.unregisterConnection(ctx, worker)
}

func (x *WebsocketManager) eventQueueOverflowContext(clientID string, worker *clientEventWorker) EventCtx {
	if worker == nil {
		return EventCtx{From: clientID}
	}
	return EventCtx{From: clientID, FromCtx: worker.snapshotFromCtx()}
}

// Handler 开始处理websocket请求
func (x *WebsocketManager) Handler(w http.ResponseWriter, r *http.Request, responseHeader http.Header) {
	if x.closed.Load() {
		http.Error(w, "websocket manager closed", http.StatusServiceUnavailable)
		return
	}
	ws, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		x.Log("[WebsocketUpgradeError]")
		return
	}
	var clientId = UUID()

	if !x.beginOperation() {
		_ = ws.Close()
		return
	}
	defer x.endOperation()
	worker := x.registerConnectionSync(EventCtx{
		From:   clientId,
		Socket: ws,
		FromCtx: &FromCtx{
			Uid:        "",
			Groups:     nil,
			UserAgent:  r.Header.Get("User-Agent"),
			RequestURI: r.RequestURI,
			Addr: AddrCtx{
				RemoteAddr:    r.RemoteAddr,
				XRealIP:       r.Header.Get("X-Real-IP"),
				XForwardedFor: r.Header.Get("X-Forwarded-For"),
			},
		},
	})
	if worker == nil {
		_ = ws.Close()
		return
	}
	x.readWG.Add(1)
	go func() {
		defer x.readWG.Done()
		x.readMessage(clientId, ws, worker)
	}()

}

// 接受请求并转给handler处理
func (x *WebsocketManager) readMessage(clientId string, ws *websocket.Conn, worker *clientEventWorker) {
	defer func() { _ = ws.Close() }()
	ws.SetReadLimit(x.Config.ReadLimitBytes)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(appData string) error {
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			// 连接故障，需要将连接上下文数据返回
			fromCtx := worker.snapshotFromCtx()
			x.unregisterConnectionSync(EventCtx{From: clientId, FromCtx: fromCtx, Socket: nil}, worker)
			break
		}
		x.logInboundMessage(messageType, data)

		var p EventCtx
		if err := json.Unmarshal(data, &p); err != nil {
			x.logProtocolError(data)
			continue
		}
		p.From = clientId
		if !x.dispatchConnectionUserEvent(worker, p.Event, p, false) {
			x.disconnectEventQueueOverflow(clientId, worker)
			break
		}
	}
}

// On 注册事件；目前支持 EventConnect EventClose EventBindUid EventUnbindUid EventJoinGroup EventLeaveGroup EventSendToClient EventSendToGroup EventSendToUid EventBroadcast (registerEvent 中所有事件)
func (x *WebsocketManager) On(eventName string, f EventHandler) bool {
	if len(eventName) < 1 {
		return false
	}
	if !x.beginOperation() {
		return false
	}
	defer x.endOperation()
	x.handlerMu.Lock()
	defer x.handlerMu.Unlock()
	if x.userEventHandlers == nil {
		x.userEventHandlers = make(map[string]EventHandler)
	}
	x.userEventHandlers[eventName] = f
	return true
}

func (x *WebsocketManager) Log(format string, v ...interface{}) {
	if x.Config.Debug {
		log.Println(fmt.Sprintf(format, v...))
	}
}

func (x *WebsocketManager) logInboundMessage(messageType int, data []byte) {
	if x.Config.LogPayload {
		x.Log("[WebsocketRequest] %d, %s", messageType, string(data))
		return
	}
	x.Log("[WebsocketRequest] type=%d bytes=%d", messageType, len(data))
}

func (x *WebsocketManager) logProtocolError(data []byte) {
	if x.Config.LogPayload {
		x.Log("[WebsocketRequestProtocolError] %s", string(data))
		return
	}
	x.Log("[WebsocketRequestProtocolError] bytes=%d", len(data))
}

func (x *WebsocketManager) LogForce(format string, v ...interface{}) {
	log.Println(fmt.Sprintf(format, v...))
}

func (x *WebsocketManager) ToBytes(v interface{}) []byte {
	buff, _ := json.MarshalIndent(v, "", "\t")
	return buff
}
