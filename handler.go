package goWebsocket

import (
	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
	"time"
)

func (x *WebsocketManager) R(ctx ClientCtx) {
	x.registerHandler(ctx)
}

func (x *WebsocketManager) registerHandler(ctx ClientCtx) {
	if _, ok := x.clients.Get(ctx.Id); !ok {
		x.clients.Set(ctx.Id, ConnectionCtx{
			Socket: ctx.Socket,
			Group:  cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]]{},
			Uid:    "",
		})
	}
}

func (x *WebsocketManager) unregisterHandler(ctx ClientCtx) {
	x.clients.RemoveCb(ctx.Id, func(key string, v ConnectionCtx, exists bool) bool {
		return true
	})
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, messageType int, data []byte) bool {
	if v, ok := x.clients.Get(clientId); ok {
		if err := v.Socket.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return false
		}
		if err := v.Socket.WriteMessage(messageType, data); err != nil {
			return false
		}
		return true
	}
	return false
}

func (x *WebsocketManager) BindUid(clientId, uid string) bool {
	if len(clientId) == 0 || len(uid) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	var tmpConn = v
	var prevUid = tmpConn.Uid

	tmpConn.Uid = uid
	x.clients.Set(clientId, tmpConn)

	var tmpU cmap.ConcurrentMap[string, bool]

	if len(prevUid) > 0 && prevUid != uid {
		// 删除旧Uid
		u, ok := x.users.Get(prevUid)
		if ok {
			tmpU = u
		} else {
			tmpU = cmap.New[bool]()
		}
		tmpU.Remove(clientId)
		x.users.Set(prevUid, tmpU)
	}

	// 绑定新Uid
	u, ok := x.users.Get(uid)
	if ok {
		tmpU = u
	} else {
		tmpU = cmap.New[bool]()
	}
	tmpU.Set(clientId, true)
	x.users.Set(uid, tmpU)

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

func (x *WebsocketManager) ListGroup() map[string]map[string]bool {
	var groups = make(map[string]map[string]bool)
	for tmpGroup, tmpValue := range x.groups.Items() {
		if groups[tmpGroup] == nil {
			groups[tmpGroup] = make(map[string]bool)
		}
		for tmpId, b := range tmpValue.Items() {
			groups[tmpGroup][tmpId] = b
		}
	}
	return groups
}

func (x *WebsocketManager) ListUser() map[string]map[string]bool {
	var uid = make(map[string]map[string]bool)
	for tmpUid, tmpValue := range x.users.Items() {
		if uid[tmpUid] == nil {
			uid[tmpUid] = make(map[string]bool)
		}
		for tmpId, b := range tmpValue.Items() {
			uid[tmpUid][tmpId] = b
		}
	}
	return uid
}

func (x *WebsocketManager) ListConn() map[string]ConnectionCtxPlain {
	var conn = make(map[string]ConnectionCtxPlain)
	for tmpId, tmpValue := range x.clients.Items() {
		v, ok := conn[tmpId]
		if !ok {
			v = ConnectionCtxPlain{Group: make(map[string]bool)}
		}
		v.Uid = tmpValue.Uid

		if !tmpValue.Group.IsNil() {
			for tmpGroupId, _ := range tmpValue.Group.Items() {
				v.Group[tmpGroupId] = true
			}
		}
		conn[tmpId] = v
	}
	return conn
}
