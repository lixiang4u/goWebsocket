package goWebsocket

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
	"log"
	"net/http"
	"time"
)

const (
	pongWait    = 60 * time.Second
	writeWait   = 45 * time.Second
	writeTicker = 45 * time.Second

	readLimitSize   = 1024
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

type WebsocketManager struct {
	Config struct {
		Debug bool
	}

	eventHandlers     map[string]EventHandler
	userEventHandlers map[string]EventHandler

	clients cmap.ConcurrentMap[string, ConnectionCtx]                    // [ClientId => ConnectionCtx]
	users   cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]] // [Uid => ClientMapEmpty]
	groups  cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]] // [GroupName => ClientMapEmpty]

	// events
	register    chan EventCtx
	unregister  chan EventCtx
	bind        chan EventCtx
	unbind      chan EventCtx
	join        chan EventCtx
	leave       chan EventCtx
	send        chan EventCtx
	sendToGroup chan EventCtx
	sendToUid   chan EventCtx
	broadcast   chan EventCtx
}

func NewWebsocketManager(debug ...bool) *WebsocketManager {
	var x = new(WebsocketManager)
	if len(debug) > 0 {
		x.Config.Debug = debug[0]
	}

	x.register = make(chan EventCtx)
	x.unregister = make(chan EventCtx)
	x.bind = make(chan EventCtx)
	x.unbind = make(chan EventCtx)
	x.join = make(chan EventCtx)
	x.leave = make(chan EventCtx)
	x.send = make(chan EventCtx)
	x.sendToGroup = make(chan EventCtx)
	x.sendToUid = make(chan EventCtx)
	x.broadcast = make(chan EventCtx)

	x.clients = cmap.New[ConnectionCtx]()
	x.users = cmap.New[cmap.ConcurrentMap[string, bool]]()
	x.groups = cmap.New[cmap.ConcurrentMap[string, bool]]()

	go x.registerEvent()

	return x
}

func (x *WebsocketManager) registerEvent() {
	for {
		select {
		case ctx := <-x.register:
			x.connect(ctx)
			x.dispatchUserEvent(Event(EventConnect).String(), ctx)
		case ctx := <-x.unregister:
			x.disconnect(ctx)
			x.dispatchUserEvent(Event(EventClose).String(), ctx)
		case ctx := <-x.bind:
			x.bindUid(ctx.From, ctx.ToUid)
			x.dispatchUserEvent(Event(EventBindUid).String(), ctx)
		case ctx := <-x.unbind:
			x.unbindUid(ctx.From, ctx.ToUid)
			x.dispatchUserEvent(Event(EventUnbindUid).String(), ctx)
		case ctx := <-x.join:
			x.joinGroup(ctx.From, ctx.ToGroup)
			x.dispatchUserEvent(Event(EventJoinGroup).String(), ctx)
		case ctx := <-x.leave:
			x.leaveGroup(ctx.From, ctx.ToGroup)
			x.dispatchUserEvent(Event(EventLeaveGroup).String(), ctx)
		case ctx := <-x.send:
			x._send(ctx.ToId, websocket.TextMessage, ctx.Data)
		case ctx := <-x.sendToGroup:
			if tmpGroup, ok := x.groups.Get(ctx.ToGroup); ok {
				if !tmpGroup.IsEmpty() {
					tmpGroup.IterCb(func(tmpClientId string, v bool) {
						x._send(tmpClientId, websocket.TextMessage, ctx.Data)
					})
				}
			}
		case ctx := <-x.sendToUid:
			if tmpUser, ok := x.users.Get(ctx.ToUid); ok {
				if !tmpUser.IsEmpty() {
					tmpUser.IterCb(func(tmpClientId string, v bool) {
						x._send(tmpClientId, websocket.TextMessage, ctx.Data)
					})
				}
			}
		case ctx := <-x.broadcast:
			x.clients.IterCb(func(tmpClientId string, v ConnectionCtx) {
				x._send(tmpClientId, websocket.TextMessage, ctx.Data)
			})
		}
	}
}

func (x *WebsocketManager) dispatchUserEvent(eventName string, ctx EventCtx) {
	if v, ok := x.userEventHandlers[eventName]; ok && v != nil {
		go v(ctx)
	}
}

// Handler 开始处理websocket请求
func (x *WebsocketManager) Handler(w http.ResponseWriter, r *http.Request, responseHeader http.Header) {
	ws, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		x.Log("[WebsocketUpgradeError] %s", err.Error())
		return
	}
	var clientId = UUID()

	go x.writeMessage(clientId, ws)
	go x.readMessage(clientId, ws)

	x.Connect(EventCtx{From: clientId, Socket: ws})

}

// 接受请求并转给handler处理
func (x *WebsocketManager) readMessage(clientId string, ws *websocket.Conn) {
	defer func() { _ = ws.Close() }()
	ws.SetReadLimit(readLimitSize)
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(appData string) error {
		_ = ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		messageType, data, err := ws.ReadMessage()
		if err != nil {
			// 连接故障
			x.Disconnect(EventCtx{From: clientId, Socket: ws})
			break
		}
		x.Log("[WebsocketRequest] %d, %s", messageType, string(data))

		var p EventCtx
		if err := json.Unmarshal(data, &p); err != nil {
			x.Log("[WebsocketRequestProtocolError] %s", string(data))
			continue
		}
		p.From = clientId
		x.dispatchUserEvent(p.Event, p)
	}
}

// 其实就是心跳逻辑
func (x *WebsocketManager) writeMessage(clientId string, ws *websocket.Conn) {
	defer func() { _ = ws.Close() }()

	ticker := time.NewTicker(writeTicker)
	defer ticker.Stop()

EXIT:
	for {
		select {
		case <-ticker.C:
			// 检测是否已经在 ReadMessage 时断开，如果是需要跳出 WriteMessage 循环
			if _, ok := x.clients.Get(clientId); !ok {
				x.Log("[WebsocketTickerWriteError] %s, %s", clientId, "NOT EXISTS")
				break EXIT
			}
			if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				x.Log("[WebsocketTickerWriteError] %s, %s", clientId, err.Error())
				break EXIT
			}
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				x.Log("[WebsocketTickerWriteError] %s, %s", clientId, err.Error())
				break EXIT
			}
		}
	}
}

// On 注册事件；目前支持 EventConnect EventClose EventBindUid EventUnbindUid EventJoinGroup EventLeaveGroup EventSendToClient EventSendToGroup EventSendToUid EventBroadcast (registerEvent 中所有事件)
func (x *WebsocketManager) On(eventName string, f EventHandler) bool {
	if len(eventName) < 1 {
		return false
	}
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

func (x *WebsocketManager) LogForce(format string, v ...interface{}) {
	log.Println(fmt.Sprintf(format, v...))
}

func (x *WebsocketManager) ToBytes(v interface{}) []byte {
	buff, _ := json.MarshalIndent(v, "", "\t")
	return buff
}
