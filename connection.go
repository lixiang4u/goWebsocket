package goWebsocket

import (
	cmap "github.com/lixiang4u/concurrent-map"
	"time"
)

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

func (x *WebsocketManager) registerHandler(ctx EventCtx) {
	if _, ok := x.clients.Get(ctx.Id); !ok {
		x.clients.Set(ctx.Id, ConnectionCtx{
			Socket: ctx.Socket,
			Group:  make(map[string]bool),
			Uid:    "",
		})
	}
}

func (x *WebsocketManager) unregisterHandler(ctx EventCtx) {
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

func (x *WebsocketManager) bindUid(clientId, uid string) bool {
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

func (x *WebsocketManager) unbindUid(clientId, uid string) bool {
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

func (x *WebsocketManager) joinGroup(clientId, group string) bool {
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

func (x *WebsocketManager) leaveGroup(clientId, group string) bool {
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
