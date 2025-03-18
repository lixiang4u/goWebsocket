package goWebsocket

import (
	"github.com/gorilla/websocket"
	"time"
)

func (x *WebsocketManager) registerHandler(ctx ClientCtx) {
	if _, ok := x.clients.Load(ctx.Id); !ok {
		x.clients.Store(ctx.Id, ConnectionCtx{
			Socket: ctx.Socket,
			Group:  make(map[string]bool),
			Uid:    "",
		})
	}
}

func (x *WebsocketManager) unregisterHandler(ctx ClientCtx) {
	if _, ok := x.clients.Load(ctx.Id); ok {
		x.clients.Delete(ctx.Id)
	}
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, messageType int, data []byte) bool {
	if v, ok := x.clients.Load(clientId); ok {
		if err := v.(ClientCtx).Socket.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return false
		}
		if err := v.(ClientCtx).Socket.WriteMessage(messageType, data); err != nil {
			return false
		}
		return true
	}
	return false
}

func (x *WebsocketManager) BindUid(clientId, uid string) bool {
	v, ok := x.clients.Load(clientId)
	if !ok {
		return false
	}
	var tmpConn = v.(ConnectionCtx)
	var prevUid = tmpConn.Uid

	tmpConn.Uid = uid
	x.clients.Store(clientId, tmpConn)

	var tmpU = ClientMapEmpty{}
	u, ok := x.users.Load(prevUid)
	if ok {
		tmpU = u.(ClientMapEmpty)
	}
	delete(tmpU, clientId)
	x.users.Store(prevUid, tmpU)

	return true
}

func (x *WebsocketManager) eventHelpHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventConnectHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventCloseHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventStatHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventBindUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventPingHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventBroadcastHandler(clientId string, ws *websocket.Conn, messageType int, data MessageProtocol) bool {
	x.send <- data
	return true
}

func (x *WebsocketManager) eventJoinGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventLeaveGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventListGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventListGroupClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true

}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, messageType int, data []byte) bool {
	return true
}

func (x *WebsocketManager) SendToUid(uid string, messageType int, data []byte) bool {
	return true
}

// =====================================================================================

func (x *WebsocketManager) ListGroup() map[string]ClientMapEmpty {
	var groups = make(map[string]ClientMapEmpty)
	x.groups.Range(func(key, value any) bool {
		var tmpKey = key.(string)
		if groups[tmpKey] == nil {
			groups[tmpKey] = make(ClientMapEmpty)
		}
		for tmpClientId, b := range value.(ClientMapEmpty) {
			groups[tmpKey][tmpClientId] = b
		}
		return true
	})
	return groups
}

func (x *WebsocketManager) ListUser() map[string]ClientMapEmpty {
	var uid = make(map[string]ClientMapEmpty)
	x.users.Range(func(key, value any) bool {
		var tmpKey = key.(string)
		if uid[tmpKey] == nil {
			uid[tmpKey] = make(ClientMapEmpty)
		}
		for tmpClientId, b := range value.(ClientMapEmpty) {
			uid[tmpKey][tmpClientId] = b
		}
		return true
	})
	return uid
}

func (x *WebsocketManager) ListConn() map[string]ConnectionCtx {
	var conn = make(map[string]ConnectionCtx)
	x.clients.Range(func(key, value any) bool {
		var tmpKey = key.(string)
		v, ok := conn[tmpKey]
		if !ok {
			conn[tmpKey] = ConnectionCtx{}
		}
		v.Uid = value.(ConnectionCtx).Uid
		for tmpClientId, b := range value.(ConnectionCtx).Group {
			v.Group[tmpClientId] = b
		}
		conn[tmpKey] = v
		return true
	})
	return conn
}
