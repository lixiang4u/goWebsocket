package go_websocket

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
	"time"
)

const (
	pongWait    = 60 * time.Second
	writeWait   = 10 * time.Second
	writeTicker = 10 * time.Second

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

var conn connections

// 事件响应格式
type eventHandler func(clientId string, ws *websocket.Conn, messageType int, data []byte) bool

// ws数据交互格式，基于json，event字段必选
type protocol struct {
	ClientId string      `json:"client_id"`
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}

// ws的全局配置
type Conf struct {
	Debug bool
}

// 当前ws对外包装
type WSWrapper struct {
	eventHandlers sync.Map
	Config        Conf
}

// 全局连接信息
type connections struct {
	Conn sync.Map
}

// iris版本的启动
//func (c *WSWrapper) RunWithIris(ctx iris.Context) {
//	c.Run(ctx.ResponseWriter(), ctx.Request(), nil)
//}

func (c *WSWrapper) Run(w http.ResponseWriter, r *http.Request, responseHeader http.Header) {
	ws, err := upgrader.Upgrade(w, r, responseHeader)
	if err != nil {
		Log2("[WebsocketUpgradeError] %s", err.Error())
		return
	}

	// 记录用户
	var clientId = UUID()
	conn.Conn.Store(clientId, ws)

	go c.writeMessage(clientId, ws)
	go c.readMessage(clientId, ws)

}

// 接受请求并转给handler处理
func (c *WSWrapper) readMessage(clientId string, ws *websocket.Conn) {
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
			if c.Config.Debug {
				Log2("[WebsocketError] %s", err.Error())
			}
			conn.Conn.Delete(clientId)
			break
		}
		if c.Config.Debug {
			Log2("[WebsocketRequest] %s", string(data))
		}

		var p protocol
		if err := json.Unmarshal(data, &p); err != nil {
			if c.Config.Debug {
				Log2("[WebsocketRequestProtocolError] %s", string(data))
			}
			continue
		}
		v, ok := c.eventHandlers.Load(p.Event)
		if !ok {
			if c.Config.Debug {
				Log2("[WebsocketEventNotFound] %s", p.Event)
			}
			continue
		}
		if v.(eventHandler) == nil {
			if c.Config.Debug {
				Log2("[WebsocketEventHandlerNil] %s", p.Event)
			}
			continue
		}
		p.ClientId = clientId
		data, _ = json.MarshalIndent(p, "", "	")
		v.(eventHandler)(clientId, ws, messageType, data)
	}
}

// 其实就是心跳逻辑
func (c *WSWrapper) writeMessage(clientId string, ws *websocket.Conn) {
	defer func() { _ = ws.Close() }()

	ticker := time.NewTicker(writeTicker)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ws.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				if c.Config.Debug {
					Log2("[WebsocketTickerWriteError] %s", err.Error())
				}
				return
			}
			if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				if c.Config.Debug {
					Log2("[WebsocketTickerWriteError] %s", err.Error())
				}
				return
			}
		}
	}
}

// 注册事件
func (c *WSWrapper) On(eventName string, f eventHandler) bool {
	if len(eventName) < 1 {
		return false
	}
	if _, ok := c.eventHandlers.Load(eventName); ok {
		return true
	}
	c.eventHandlers.Store(eventName, f)

	return true
}

// 对外接口，用于发送ws消息到指定clientId
func WSendMessage(clientId string, messageType int, data []byte) bool {
	v, ok := conn.Conn.Load(clientId)
	if !ok {
		return false
	}
	if v.(*websocket.Conn) == nil {
		return false
	}
	if err := v.(*websocket.Conn).SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return false
	}
	if err := v.(*websocket.Conn).WriteMessage(messageType, data); err != nil {
		return false
	}
	return true
}

func WSConnectionList() map[string]interface{} {
	var data = make(map[string]interface{})
	conn.Conn.Range(func(key, value interface{}) bool {
		data[key.(string)] = value.(*websocket.Conn).UnderlyingConn().RemoteAddr().String()
		return true
	})
	return data
}

func WSBroadcast(clientId string, messageType int, data []byte) {
	conn.Conn.Range(func(key, value interface{}) bool {
		//跳过广播发送给自己
		if key.(string) == clientId {
			return true
		}
		_ = value.(*websocket.Conn).WriteMessage(messageType, data)
		return true
	})

}
