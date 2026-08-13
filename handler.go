package goWebsocket

import (
	"encoding/json"
	"sort"

	"github.com/gorilla/websocket"
)

type recipientSnapshot struct {
	clientID string
	worker   *clientEventWorker
	valid    bool
}

func (x *WebsocketManager) Connect(ctx EventCtx) {
	x.submitEvent(x.register, ctx)
}

func (x *WebsocketManager) Disconnect(ctx EventCtx) {
	request := connectionUnregistration{ctx: ctx}
	x.submitUnregister(request)
}

func (x *WebsocketManager) BindUid(clientId, uid string) {
	x.submitEvent(x.bind, EventCtx{From: clientId, ToUid: uid})
}

func (x *WebsocketManager) UnbindUid(clientId, uid string) {
	x.submitEvent(x.unbind, EventCtx{From: clientId, ToUid: uid})
}

func (x *WebsocketManager) JoinGroup(clientId, group string) {
	x.submitEvent(x.join, EventCtx{From: clientId, ToGroup: group})
}

func (x *WebsocketManager) LeaveGroup(clientId, group string) {
	x.submitEvent(x.leave, EventCtx{From: clientId, ToGroup: group})
}

func (x *WebsocketManager) submitEvent(channel chan<- EventCtx, ctx EventCtx) bool {
	if !x.beginOperation() {
		return false
	}
	defer x.endOperation()
	select {
	case <-x.context.Done():
		return false
	case channel <- ctx:
		return true
	}
}

func (x *WebsocketManager) submitUnregister(request connectionUnregistration) bool {
	if !x.beginOperation() {
		return false
	}
	defer x.endOperation()
	select {
	case <-x.context.Done():
		return false
	case x.unregister <- request:
		return true
	}
}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, data interface{}) {
	if x.submitLegacyRequest(x.sendSync, EventCtx{ToId: clientId, Data: data}) {
		x.dispatchLegacyUserEvent(Event(EventSendToClient).String(), EventCtx{ToId: clientId, Data: data})
	}
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, data interface{}) {
	if x.submitLegacyRequest(x.sendGroupSync, EventCtx{ToGroup: groupName, Data: data}) {
		x.dispatchLegacyUserEvent(Event(EventSendToGroup).String(), EventCtx{ToGroup: groupName, Data: data})
	}
}

func (x *WebsocketManager) SendToUid(uid string, data interface{}) {
	if x.submitLegacyRequest(x.sendUidSync, EventCtx{ToUid: uid, Data: data}) {
		x.dispatchLegacyUserEvent(Event(EventSendToUid).String(), EventCtx{ToUid: uid, Data: data})
	}
}

func (x *WebsocketManager) SendToAll(data interface{}) {
	if x.submitLegacyRequest(x.broadcastSync, EventCtx{Data: data}) {
		x.dispatchLegacyUserEvent(Event(EventBroadcast).String(), EventCtx{Data: data})
	}
}

func (x *WebsocketManager) submitLegacyRequest(channel chan<- sendRequest, ctx EventCtx) bool {
	if !x.beginOperation() {
		return false
	}
	defer x.endOperation()
	done := make(chan struct{})
	select {
	case <-x.context.Done():
		return false
	case channel <- sendRequest{ctx: ctx, done: done}:
	}
	<-done
	return true
}

// EnqueueToGroup marshals data once and attempts a non-blocking enqueue for a
// sorted snapshot of the group's connection identities.
func (x *WebsocketManager) EnqueueToGroup(group string, data interface{}) EnqueueResult {
	if !x.beginOperation() {
		return emptyEnqueueResult()
	}
	defer x.endOperation()
	return x.enqueueToGroup(group, data)
}

func (x *WebsocketManager) enqueueToGroup(group string, data interface{}) EnqueueResult {
	payload, ok := x.marshalOutbound(data)
	if !ok {
		return emptyEnqueueResult()
	}
	return x.enqueueRecipients(x.snapshotGroupRecipients(group), websocket.TextMessage, payload)
}

func emptyEnqueueResult() EnqueueResult {
	return EnqueueResult{Accepted: make([]string, 0), Rejected: make([]string, 0)}
}

func (x *WebsocketManager) marshalOutbound(data interface{}) ([]byte, bool) {
	payload, err := json.Marshal(data)
	if err != nil {
		x.Log("[WebsocketMarshalError]")
		return nil, false
	}
	return payload, true
}

func (x *WebsocketManager) snapshotGroupRecipients(group string) []recipientSnapshot {
	members, ok := x.groups.Get(group)
	if !ok || members.IsNil() {
		return make([]recipientSnapshot, 0)
	}
	return x.snapshotClientRecipients(sortedMemberIDs(members.Items()))
}

func (x *WebsocketManager) snapshotUserRecipients(uid string) []recipientSnapshot {
	members, ok := x.users.Get(uid)
	if !ok || members.IsNil() {
		return make([]recipientSnapshot, 0)
	}
	return x.snapshotClientRecipients(sortedMemberIDs(members.Items()))
}

func (x *WebsocketManager) snapshotAllRecipients() []recipientSnapshot {
	items := x.clients.Items()
	clientIDs := make([]string, 0, len(items))
	for clientID := range items {
		clientIDs = append(clientIDs, clientID)
	}
	sort.Strings(clientIDs)
	return x.snapshotClientRecipients(clientIDs)
}

func sortedMemberIDs(items map[string]bool) []string {
	clientIDs := make([]string, 0, len(items))
	for clientID := range items {
		clientIDs = append(clientIDs, clientID)
	}
	sort.Strings(clientIDs)
	return clientIDs
}

func (x *WebsocketManager) snapshotClientRecipients(clientIDs []string) []recipientSnapshot {
	recipients := make([]recipientSnapshot, 0, len(clientIDs))
	for _, clientID := range clientIDs {
		recipient := recipientSnapshot{clientID: clientID}
		record, recordOK := x.connectionRecords.Get(clientID)
		connection, published := x.clients.Get(clientID)
		if recordOK && published && record.ready && record.writer != nil && record.worker != nil && connection.Socket == record.socket {
			recipient.worker = record.worker
			recipient.valid = true
		}
		recipients = append(recipients, recipient)
	}
	return recipients
}

func (x *WebsocketManager) enqueueRecipients(recipients []recipientSnapshot, messageType int, payload []byte) EnqueueResult {
	result := EnqueueResult{
		Accepted: make([]string, 0, len(recipients)),
		Rejected: make([]string, 0, len(recipients)),
	}
	for _, recipient := range recipients {
		if recipient.valid && x.enqueueClient(recipient.clientID, recipient.worker, messageType, payload) {
			result.Accepted = append(result.Accepted, recipient.clientID)
			continue
		}
		result.Rejected = append(result.Rejected, recipient.clientID)
	}
	return result
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
