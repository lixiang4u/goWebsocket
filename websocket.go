package goWebsocket

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"slices"
	"sync"
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

// 阻止部分敏感操作，应由后台验证权限后替代操作
var blockSensitiveEvents = []string{
	Event(EventBroadcast).String(),
	Event(EventBindUid).String(),
	Event(EventSendToUid).String(),
	Event(EventListGroup).String(),
}

type H map[string]interface{}

type WebsocketManager struct {
	eventHandlers     map[string]EventHandler
	userEventHandlers map[string]EventHandler
	Conn              *ConnectionMutex
	Config            struct {
		Debug bool
	}
}

func NewWebsocketManager(debug ...bool) *WebsocketManager {
	var x = new(WebsocketManager)
	if len(debug) > 0 {
		x.Config.Debug = debug[0]
	}
	x.Conn = &ConnectionMutex{
		Conn:  make(map[string]*ConnectionContext),
		Uid:   make(map[string]map[string]bool),
		Group: make(map[string]map[string]bool),
		mutex: sync.RWMutex{},
	}
	x.registerEvents()
	return x
}

// iris版本的启动
//func (x *WebsocketManager) RunWithIris(ctx iris.Context) {
//	c.Run(ctx.ResponseWriter(), ctx.Request(), nil)
//}

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

	x.eventConnectHandler(clientId, ws, 0, EventProtocol{
		ClientId: clientId,
		Event:    Event(EventConnect).String(),
		Data:     nil,
	})
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
			x.eventCloseHandler(clientId, ws, messageType, EventProtocol{
				ClientId: clientId,
				Event:    Event(EventClose).String(),
				Data:     nil,
			})
			break
		}
		x.Log("[WebsocketRequest] %s", string(data))

		var p EventProtocol
		if err := json.Unmarshal(data, &p); err != nil {
			x.Log("[WebsocketRequestProtocolError] %s", string(data))
			continue
		}
		p.ClientId = clientId

		// 先执行内置事件（同步操作），在执行用户事件（异步）
		var runNext = true
		if !slices.Contains(blockSensitiveEvents, p.Event) {
			if v, ok := x.eventHandlers[p.Event]; ok && v != nil {
				runNext = v(clientId, ws, messageType, p)
			}
		}
		if v, ok := x.userEventHandlers[p.Event]; ok && v != nil && runNext {
			go v(clientId, ws, messageType, p)
		}
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
			if x.Conn.LoadConn(clientId) == nil {
				//x.Log("[WebsocketTickerWriteError] %s, %s", clientId, "NOT EXISTS")
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

// On 注册事件
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

func (x *WebsocketManager) registerEvents() {
	if x.eventHandlers == nil {
		x.eventHandlers = make(map[string]EventHandler)
	}
	x.eventHandlers[Event(EventHelp).String()] = x.eventHelpHandler
	x.eventHandlers[Event(EventConnect).String()] = x.eventConnectHandler
	x.eventHandlers[Event(EventClose).String()] = x.eventCloseHandler
	x.eventHandlers[Event(EventStat).String()] = x.eventStatHandler
	x.eventHandlers[Event(EventPing).String()] = x.eventPingHandler
	x.eventHandlers[Event(EventBindUid).String()] = x.eventBindUidHandler
	x.eventHandlers[Event(EventSendToClient).String()] = x.eventSendToClientHandler
	x.eventHandlers[Event(EventSendToUid).String()] = x.eventSendToUidHandler
	x.eventHandlers[Event(EventSendToGroup).String()] = x.eventSendToGroupHandler
	x.eventHandlers[Event(EventBroadcast).String()] = x.eventBroadcastHandler
	x.eventHandlers[Event(EventJoinGroup).String()] = x.eventJoinGroupHandler
	x.eventHandlers[Event(EventLeaveGroup).String()] = x.eventLeaveGroupHandler
	x.eventHandlers[Event(EventListGroup).String()] = x.eventListGroupHandler
	x.eventHandlers[Event(EventListGroupClient).String()] = x.eventListGroupClientHandler
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
