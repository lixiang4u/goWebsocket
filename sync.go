package goWebsocket

import cmap "github.com/lixiang4u/concurrent-map"

// 直接对users、groups操作加锁

func (x *WebsocketManager) _addUserOp(ctx SeqOpCtx) {
	if len(ctx.ClientId) == 0 || len(ctx.Uid) == 0 {
		return
	}
	x.Lock()
	defer x.Unlock()

	v, ok := x.users.Get(ctx.Uid)
	if !ok {
		v = cmap.New[bool]()
	}
	v.Set(ctx.ClientId, true)
	x.users.Set(ctx.Uid, v)
}

func (x *WebsocketManager) _removeUserOp(ctx SeqOpCtx) {
	if len(ctx.ClientId) == 0 || len(ctx.Uid) == 0 {
		return
	}

	x.Lock()
	defer x.Unlock()

	v, ok := x.users.Get(ctx.Uid)
	if !ok {
		return
	}
	v.Remove(ctx.ClientId)
	x.users.Set(ctx.Uid, v)
}
func (x *WebsocketManager) _addGroupOp(ctx SeqOpCtx) {
	x.Lock()
	defer x.Unlock()

	if len(ctx.ClientId) == 0 || len(ctx.Group) == 0 {
		return
	}

	v, ok := x.groups.Get(ctx.Group)
	if !ok {
		v = cmap.New[bool]()
	}
	v.Set(ctx.ClientId, true)
	x.groups.Set(ctx.Group, v)
}

func (x *WebsocketManager) _removeGroupOp(ctx SeqOpCtx) {
	if len(ctx.ClientId) == 0 || len(ctx.Uid) == 0 {
		return
	}

	x.Lock()
	defer x.Unlock()

	v, ok := x.groups.Get(ctx.Group)
	if !ok {
		return
	}
	v.Remove(ctx.ClientId)
	x.groups.Set(ctx.Group, v)
}
