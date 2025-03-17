package goWebsocket

import (
	"github.com/gorilla/websocket"
	"sync"
)

type ConnectionContext struct {
	Conn  *websocket.Conn
	Group map[string]bool
	Uid   string
}

type ConnectionMutex struct {
	Conn  map[string]ConnectionContext // [ClientId => CONNECTION_DATA]
	Uid   map[string]map[string]bool   // [Uid => [ClientId => bool]]
	Group map[string]map[string]bool   // [GroupName => [ClientId => bool]]
	mutex sync.RWMutex
}

// Store 连接时添加客户端信息
func (x *ConnectionMutex) Store(clientId string, ws *websocket.Conn) {
	x.mutex.Lock()
	defer x.mutex.Unlock()
	if _, ok := x.Conn[clientId]; !ok {
		x.Conn[clientId] = ConnectionContext{
			Conn:  ws,
			Group: make(map[string]bool),
			Uid:   "",
		}
	}
}

// Remove 断开连接时删除客户端信息
func (x *ConnectionMutex) Remove(clientId string) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	if v, ok := x.Conn[clientId]; ok {
		delete(x.Conn, clientId)
		delete(x.Uid, v.Uid)
		for tmpGroup, _ := range v.Group {
			if x.Group[tmpGroup] != nil {
				delete(x.Group[tmpGroup], clientId)
			}
		}
	}
}

// BindUid 绑定用户ID
func (x *ConnectionMutex) BindUid(clientId, uid string) bool {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	v, ok := x.Conn[clientId]
	if !ok {
		return false
	}
	// 解绑当前连接之前的Uid
	if x.Uid[uid] != nil {
		delete(x.Uid[uid], clientId)
	}

	// 绑定新Uid到当前连接
	v.Uid = uid
	x.Conn[clientId] = v

	if x.Uid[uid] == nil {
		x.Uid[uid] = make(map[string]bool)
	}
	x.Uid[uid][clientId] = true

	return true
}

// UnbindUid 解绑指定连接的Uid
func (x *ConnectionMutex) UnbindUid(clientId, uid string) bool {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	v, ok := x.Conn[clientId]
	if !ok {
		return false
	}
	v.Uid = ""
	x.Conn[clientId] = v

	if x.Uid[uid] != nil {
		delete(x.Uid[uid], clientId)
	}
	return true
}

func (x *ConnectionMutex) GetUidClientId(uid string) []string {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Uid[uid] {
		clientIds = append(clientIds, clientId)
	}

	return clientIds
}

func (x *ConnectionMutex) JoinGroup(clientId, groupName string) bool {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	v, ok := x.Conn[clientId]
	if !ok {
		return false
	}

	if v.Group == nil {
		v.Group = make(map[string]bool)
	}
	v.Group[groupName] = true
	x.Conn[clientId] = v

	v2, ok := x.Group[groupName]
	if !ok {
		v2 = make(map[string]bool)
	}
	v2[clientId] = true
	x.Group[groupName] = v2

	return true
}

func (x *ConnectionMutex) LeaveGroup(clientId, groupName string) bool {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	if _, ok := x.Conn[clientId]; !ok {
		return false
	}

	if x.Conn[clientId].Group != nil {
		delete(x.Conn[clientId].Group, groupName)
	}
	if x.Group[groupName] != nil {
		delete(x.Group[groupName], clientId)
	}

	return true
}

func (x *ConnectionMutex) ListGroup() []string {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var groups = make([]string, 0)
	for g, _ := range x.Group {
		groups = append(groups, g)
	}

	return groups
}

func (x *ConnectionMutex) ListGroupClientIds(groupName string) []string {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Group[groupName] {
		clientIds = append(clientIds, clientId)
	}
	return clientIds
}

func (x *ConnectionMutex) GetGroupClientIds(groupName string) []string {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Group[groupName] {
		clientIds = append(clientIds, clientId)
	}

	return clientIds
}

func (x *ConnectionMutex) LoadConn(clientId string) *websocket.Conn {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	if _, ok := x.Conn[clientId]; !ok {
		return nil
	}
	return x.Conn[clientId].Conn
}

func (x *ConnectionMutex) LoadConnContext(clientId string) ConnectionContext {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	return x.Conn[clientId]
}
