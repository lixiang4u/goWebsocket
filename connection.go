package go_websocket

import (
	"github.com/gorilla/websocket"
	"log"
	"sync"
)

type ConnectionContext struct {
	Conn  *websocket.Conn
	Group map[string]bool
	Uid   string
}

//type connData struct {
//	Conn  *websocket.Conn
//	Group map[string]bool
//	Uid   string
//}

type ConnectionMutex struct {
	Conn  map[string]*ConnectionContext // [ClientId => CONNECTION_DATA]
	Uid   map[string]map[string]bool    // [Uid => [ClientId => bool]]
	Group map[string]map[string]bool    // [GroupName => [ClientId => bool]]
	mutex sync.RWMutex
}

func (x *ConnectionMutex) Store(clientId string, ws *websocket.Conn) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var tmp = x.Conn[clientId]
	if tmp == nil {
		tmp = new(ConnectionContext)
	}
	tmp.Conn = ws
	x.Conn[clientId] = tmp

	log.Println("[Store.Conn]", ToJson(x.Conn))
}

func (x *ConnectionMutex) Remove(clientId string) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	log.Println("[clientId]", clientId)
	log.Println("[X]", ToJson(x.Conn[clientId]))
	log.Println("[x.Conn]", ToJson(x.Conn))

	var tmp = x.Conn[clientId]
	delete(x.Uid[tmp.Uid], clientId)
	for g, _ := range tmp.Group {
		delete(x.Group[g], clientId)
	}
	delete(x.Conn, clientId)

	log.Println("[Remove.Conn]", ToJson(x.Conn))
}

func (x *ConnectionMutex) SetUid(clientId, uid string) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var tmp = x.Conn[clientId]
	tmp.Uid = uid
	x.Conn[clientId] = tmp

	var tmpUid = x.Uid[uid]
	if tmpUid == nil {
		tmpUid = make(map[string]bool)
	}
	tmpUid[clientId] = true
	x.Uid[uid] = tmpUid

	log.Println("[x.Uid]", ToJson(x.Uid))

}

func (x *ConnectionMutex) GetUidClientId(uid string) []string {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Uid[uid] {
		clientIds = append(clientIds, clientId)
	}

	log.Println("[GetUidClientId.uid]", uid)
	log.Println("[x.Uid]", ToJson(x.Uid))
	log.Println("[GetUidClientId.clientIds]", ToJson(clientIds))

	return clientIds
}

func (x *ConnectionMutex) JoinGroup(clientId, groupName string) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	var tmpConn = x.Conn[clientId]
	if tmpConn.Group == nil {
		tmpConn.Group = make(map[string]bool)
	}
	tmpConn.Group[groupName] = true
	x.Conn[clientId] = tmpConn

	var tmpGroup = x.Group[groupName]
	if tmpGroup == nil {
		tmpGroup = make(map[string]bool)
	}
	tmpGroup[clientId] = true
	x.Group[groupName] = tmpGroup
}

func (x *ConnectionMutex) LeaveGroup(clientId, groupName string) {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	delete(x.Conn[clientId].Group, groupName)
	delete(x.Group[groupName], clientId)
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
	if x.Conn[clientId] == nil {
		return nil
	}
	return x.Conn[clientId].Conn
}

func (x *ConnectionMutex) LoadConnContext(clientId string) *ConnectionContext {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	return x.Conn[clientId]
}
