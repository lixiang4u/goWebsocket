package goWebsocket

import (
	"time"
)

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

func (x *WebsocketManager) connect(ctx EventCtx) {
	if _, ok := x.clients.Get(ctx.From); !ok {
		x.clients.Set(ctx.From, ConnectionCtx{
			Socket: ctx.Socket,
			Group:  make(map[string]bool),
			Uid:    "",
		})
	}
}

func (x *WebsocketManager) disconnect(ctx EventCtx) {
	x.clients.RemoveCb(ctx.From, func(key string, v ConnectionCtx, exists bool) bool {
		if len(v.Uid) > 0 {
			x._removeUserOp(SeqOpCtx{
				ClientId: key,
				Uid:      v.Uid,
			})
		}
		if v.Group != nil {
			for tmpGroupName, _ := range v.Group {
				x._removeGroupOp(SeqOpCtx{
					ClientId: key,
					Group:    tmpGroupName,
				})
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

	if len(prevUid) > 0 && prevUid != uid {
		// 删除旧Uid
		x._removeUserOp(SeqOpCtx{
			ClientId: clientId,
			Uid:      prevUid,
		})
	}

	// 绑定新Uid
	x._addUserOp(SeqOpCtx{
		ClientId: clientId,
		Uid:      uid,
	})

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

	v.Uid = ""
	x.clients.Set(clientId, v)

	x._removeUserOp(SeqOpCtx{
		ClientId: clientId,
		Uid:      uid,
	})

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
	if _, ok = v.Group[group]; !ok {
		v.Group[group] = true
		x.clients.Set(clientId, v)
	}

	x._addGroupOp(SeqOpCtx{
		ClientId: clientId,
		Group:    group,
	})

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

	x._removeGroupOp(SeqOpCtx{
		ClientId: clientId,
		Group:    group,
	})

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
