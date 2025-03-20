package goWebsocket

import (
	"github.com/gorilla/websocket"
	cmap "github.com/lixiang4u/concurrent-map"
	"time"
)

type ConnectionCtx struct {
	Socket *websocket.Conn `json:"socket,omitempty"`
	Group  map[string]bool `json:"group"`
	Uid    string          `json:"uid"`
}

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

type ClientCtx struct {
	Id     string // 客户端Id
	Socket *websocket.Conn
}

type MessageCtx struct {
	Id  string // 客户端Id
	Msg []byte
}

type CmdCtx struct {
	Id   string // 客户端Id
	Cmd  int
	Data any
}

func (x *WebsocketManager) registerHandler(ctx ClientCtx) {
	if _, ok := x.clients.Get(ctx.Id); !ok {
		x.clients.Set(ctx.Id, ConnectionCtx{
			Socket: ctx.Socket,
			Group:  make(map[string]bool),
			Uid:    "",
		})
	}
}

func (x *WebsocketManager) unregisterHandler(ctx ClientCtx) {
	x.clients.RemoveCb(ctx.Id, func(key string, v ConnectionCtx, exists bool) bool {
		if len(v.Uid) > 0 {
			if tmpU, ok := x.users.Get(v.Uid); ok {
				tmpU.Remove(key)
				x.users.Set(v.Uid, tmpU)
			}
		}
		if v.Group != nil {
			for tmpGroupName, _ := range v.Group {
				if tmpG, ok := x.groups.Get(tmpGroupName); ok {
					tmpG.Remove(key)
					x.groups.Set(tmpGroupName, tmpG)
				}
			}
		}
		return true
	})
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

func (x *WebsocketManager) UnbindUid(clientId, uid string) bool {
	if len(clientId) == 0 || len(uid) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	if len(v.Uid) == 0 {
		return false
	}

	if tmpU, ok := x.users.Get(v.Uid); ok {
		tmpU.Remove(clientId)
		x.users.Set(v.Uid, tmpU)
	}

	v.Uid = ""
	x.clients.Set(clientId, v)

	return true
}

func (x *WebsocketManager) JoinGroup(clientId, group string) bool {
	if len(clientId) == 0 || len(group) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	if v.Group == nil {
		v.Group = make(map[string]bool)
	}
	if _, ok := v.Group[group]; !ok {
		v.Group[group] = true
		x.clients.Set(clientId, v)
	}

	tmpGroup, ok := x.groups.Get(group)
	if !ok {
		tmpGroup = cmap.New[bool]()
	}
	tmpGroup.Set(clientId, true)
	x.groups.Set(group, tmpGroup)

	return true
}

func (x *WebsocketManager) LeaveGroup(clientId, group string) bool {
	if len(clientId) == 0 || len(group) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}

	if v.Group != nil {
		delete(v.Group, group)
		x.clients.Set(clientId, v)
	}

	if tmpGroup, ok := x.groups.Get(group); ok {
		tmpGroup.Remove(clientId)
		x.groups.Set(group, tmpGroup)
	}

	return true
}

func (x *WebsocketManager) ListGroupClient(group string) []string {
	var clientList = make([]string, 0)
	tmpGroup, ok := x.groups.Get(group)
	if !ok {
		return clientList
	}
	if tmpGroup.IsNil() {
		return clientList
	}
	for tmpClientId, _ := range tmpGroup.Items() {
		clientList = append(clientList, tmpClientId)
	}
	return clientList
}

func (x *WebsocketManager) ListUserClient(uid string) []string {
	var clientList = make([]string, 0)
	tmpUser, ok := x.users.Get(uid)
	if !ok {
		return clientList
	}
	if tmpUser.IsNil() {
		return clientList
	}
	for tmpClientId, _ := range tmpUser.Items() {
		clientList = append(clientList, tmpClientId)
	}
	return clientList
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, data interface{}) bool {
	x.send <- MessageProtocol{ToId: clientId, Data: data}
	return true
}

func (x *WebsocketManager) _send(clientId string, messageType int, data interface{}) bool {
	if v, ok := x.clients.Get(clientId); ok && v.Socket != nil {
		if err := v.Socket.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
			return false
		}
		if err := v.Socket.WriteMessage(messageType, ToBuff(data)); err != nil {
			return false
		}
		return true
	}
	return false
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, data interface{}) bool {
	for _, tmpClientId := range x.ListGroupClient(groupName) {
		x.Send(tmpClientId, data)
	}
	return true
}

func (x *WebsocketManager) SendToUid(uid string, data interface{}) bool {
	for _, tmpClientId := range x.ListUserClient(uid) {
		x.Send(tmpClientId, data)
	}
	return true
}

func (x *WebsocketManager) SendToAll(data interface{}) bool {
	x.clients.IterCb(func(key string, v ConnectionCtx) {
		x.Send(key, data)
	})
	return true
}

// 获取列表

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

		if tmpValue.Group != nil {
			for tmpGroupId, _ := range tmpValue.Group {
				v.Group[tmpGroupId] = true
			}
		}
		conn[tmpId] = v
	}
	return conn
}

// 统计

func (x *WebsocketManager) ClientCount() int {
	return x.clients.Count()
}

func (x *WebsocketManager) UserCount() int {
	return x.users.Count()
}

func (x *WebsocketManager) GroupCount() int {
	return x.groups.Count()
}
