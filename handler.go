package goWebsocket

func (x *WebsocketManager) BindUid(clientId, uid string) {
	x.bind <- EventCtx{Id: clientId, Uid: uid}
}

func (x *WebsocketManager) UnbindUid(clientId, uid string) {
	x.unbind <- EventCtx{Id: clientId, Uid: uid}
}

func (x *WebsocketManager) JoinGroup(clientId, group string) {
	x.join <- EventCtx{Id: clientId, Group: group}
}

func (x *WebsocketManager) LeaveGroup(clientId, group string) {
	x.leave <- EventCtx{Id: clientId, Group: group}
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, data interface{}) {
	x.send <- EventCtx{Id: clientId, Data: data}
	x.dispatchUserEvent(Event(EventSendToClient).String(), EventCtx{Id: clientId, Data: data})
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, data interface{}) {
	x.sendToGroup <- EventCtx{Group: groupName, Data: data}
	x.dispatchUserEvent(Event(EventSendToGroup).String(), EventCtx{Group: groupName, Data: data})
}

func (x *WebsocketManager) SendToUid(uid string, data interface{}) {
	x.sendToUid <- EventCtx{Uid: uid, Data: data}
	x.dispatchUserEvent(Event(EventSendToUid).String(), EventCtx{Uid: uid, Data: data})
}

func (x *WebsocketManager) SendToAll(data interface{}) {
	x.broadcast <- EventCtx{Data: data}
	x.dispatchUserEvent(Event(EventBroadcast).String(), EventCtx{Data: data})
}

// 获取列表

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
