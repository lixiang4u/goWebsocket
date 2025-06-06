package goWebsocket

func (x *WebsocketManager) Connect(ctx EventCtx) {
	x.register <- ctx
}

func (x *WebsocketManager) Disconnect(ctx EventCtx) {
	x.unregister <- ctx
}

func (x *WebsocketManager) BindUid(clientId, uid string) {
	x.bind <- EventCtx{From: clientId, ToUid: uid}
}

func (x *WebsocketManager) UnbindUid(clientId, uid string) {
	x.unbind <- EventCtx{From: clientId, ToUid: uid}
}

func (x *WebsocketManager) JoinGroup(clientId, group string) {
	x.join <- EventCtx{From: clientId, ToGroup: group}
}

func (x *WebsocketManager) LeaveGroup(clientId, group string) {
	x.leave <- EventCtx{From: clientId, ToGroup: group}
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, data interface{}) {
	x.send <- EventCtx{ToId: clientId, Data: data}
	x.dispatchUserEvent(Event(EventSendToClient).String(), EventCtx{ToId: clientId, Data: data})
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, data interface{}) {
	x.sendToGroup <- EventCtx{ToGroup: groupName, Data: data}
	x.dispatchUserEvent(Event(EventSendToGroup).String(), EventCtx{ToGroup: groupName, Data: data})
}

func (x *WebsocketManager) SendToUid(uid string, data interface{}) {
	x.sendToUid <- EventCtx{ToUid: uid, Data: data}
	x.dispatchUserEvent(Event(EventSendToUid).String(), EventCtx{ToUid: uid, Data: data})
}

func (x *WebsocketManager) SendToAll(data interface{}) {
	x.broadcast <- EventCtx{Data: data}
	x.dispatchUserEvent(Event(EventBroadcast).String(), EventCtx{Data: data})
}

// 获取数据

func (x *WebsocketManager) UserExist(uid string) bool {
	if len(uid) == 0 {
		return false
	}
	_, ok := x.users.Get(uid)
	return ok
}

func (x *WebsocketManager) UidClientCount(uid string) int {
	if len(uid) == 0 {
		return 0
	}
	if v, ok := x.users.Get(uid); ok {
		return v.Count()
	}
	return 0
}

func (x *WebsocketManager) GetClientCtx(clientId string) (ctx FromCtx) {
	ctx.Groups = make([]string, 0)
	if v, ok := x.clients.Get(clientId); ok {
		ctx.Uid = v.Uid
		for tmpGroup, _ := range v.Group {
			ctx.Groups = append(ctx.Groups, tmpGroup)
		}
	}
	return
}

func (x *WebsocketManager) GetClientUid(clientId string) string {
	if v, ok := x.clients.Get(clientId); ok {
		return v.Uid
	}
	return ""
}

func (x *WebsocketManager) GetClientGroupList(clientId string) []string {
	var groups = make([]string, 0)
	if v, ok := x.clients.Get(clientId); ok {
		for tmpGroup, _ := range v.Group {
			groups = append(groups, tmpGroup)
		}
	}
	return groups
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
