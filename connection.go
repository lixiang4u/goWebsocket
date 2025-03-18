package goWebsocket

import (
	"github.com/gorilla/websocket"
	"sync"
)

type ClientMapEmpty map[string]bool

type ConnectionCtx struct {
	Socket *websocket.Conn
	Group  map[string]bool
	Uid    string
}

type DataHub struct {
	Uid   sync.Map // [Uid => ClientMapEmpty]
	Group sync.Map // [GroupName => ClientMapEmpty]
	Conn  sync.Map // [ClientId => ConnectionCtx]
}

// Store 连接时添加客户端信息
func (x *DataHub) Store(clientId string, ws *websocket.Conn) {
	if _, ok := x.Conn.Load(clientId); !ok {
		x.Conn.Store(clientId, ConnectionCtx{
			Socket: ws,
			Group:  make(map[string]bool),
			Uid:    "",
		})
	}
}

// Remove 断开连接时删除客户端信息
func (x *DataHub) Remove(clientId string) {
	v, ok := x.Conn.Load(clientId)
	if ok {
		x.Conn.Delete(clientId)
	}
	var tmpV = v.(ConnectionCtx)

	if len(tmpV.Uid) != 0 {
		x.Uid.Delete(tmpV.Uid)
	}
	for tmpGroupName, _ := range tmpV.Group {
		if tmpGroupMap, ok2 := x.Group.Load(tmpGroupName); ok2 {
			delete(tmpGroupMap.(ClientMapEmpty), clientId)
			x.Group.Store(tmpGroupName, tmpGroupMap)
		}
	}
	x.Conn.Store(clientId, tmpV)
}

// BindUid 绑定用户ID
func (x *DataHub) BindUid(clientId, uid string) bool {
	v, ok := x.Conn.Load(clientId)
	if !ok {
		return false
	}

	var tmpV = v.(ConnectionCtx)
	var prevUid = tmpV.Uid

	// 绑定新Uid到当前连接
	tmpV.Uid = uid
	x.Conn.Store(clientId, tmpV)

	// 解绑之前Uid
	if u, ok := x.Uid.Load(prevUid); ok {
		delete(u.(ClientMapEmpty), clientId)
		x.Uid.Store(prevUid, u)
	}

	// 绑定新Uid
	u, ok := x.Uid.Load(uid)
	if !ok {
		u = ClientMapEmpty{}
	}
	u.(ClientMapEmpty)[clientId] = true
	x.Uid.Store(uid, u)

	return true
}

// UnbindUid 解绑指定连接的Uid
func (x *DataHub) UnbindUid(clientId, uid string) bool {
	v, ok := x.Conn.Load(clientId)
	if !ok {
		return false
	}

	var tmpV = v.(ConnectionCtx)
	if uid == tmpV.Uid {
		tmpV.Uid = ""
	}
	x.Conn.Store(clientId, tmpV)

	if u, ok := x.Uid.Load(uid); ok {
		delete(u.(ClientMapEmpty), clientId)
		x.Uid.Store(uid, u)
	}

	return true
}

func (x *DataHub) GetUidClientId(uid string) []string {

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Uid[uid] {
		clientIds = append(clientIds, clientId)
	}

	return clientIds
}

func (x *DataHub) JoinGroup(clientId, groupName string) bool {

	//v, ok := x.Conn[clientId]
	//if !ok {
	//	return false
	//}
	//
	//if v.Group == nil {
	//	v.Group = make(map[string]bool)
	//}
	//v.Group[groupName] = true
	//x.Conn[clientId] = v
	//
	//v2, ok := x.Group[groupName]
	//if !ok {
	//	v2 = make(map[string]bool)
	//}
	//v2[clientId] = true
	//x.Group[groupName] = v2

	return true
}

func (x *DataHub) LeaveGroup(clientId, groupName string) bool {

	//if _, ok := x.Conn[clientId]; !ok {
	//	return false
	//}
	//
	//if x.Conn[clientId].Group != nil {
	//	delete(x.Conn[clientId].Group, groupName)
	//}
	//if x.Group[groupName] != nil {
	//	delete(x.Group[groupName], clientId)
	//}

	return true
}

func (x *DataHub) ListGroup() []string {

	var groups = make([]string, 0)
	for g, _ := range x.Group {
		groups = append(groups, g)
	}

	return groups
}

func (x *DataHub) ListGroupClientIds(groupName string) []string {

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Group[groupName] {
		clientIds = append(clientIds, clientId)
	}
	return clientIds
}

func (x *DataHub) GetGroupClientIds(groupName string) []string {

	var clientIds = make([]string, 0)
	for clientId, _ := range x.Group[groupName] {
		clientIds = append(clientIds, clientId)
	}

	return clientIds
}

func (x *DataHub) LoadConn(clientId string) *websocket.Conn {
	if v, ok := x.Conn.Load(clientId); ok {
		return v.(ConnectionCtx).Socket
	}
	return nil
}
