package goWebsocket

import (
	"fmt"
	"github.com/gorilla/websocket"
	"time"
)

func (x *WebsocketManager) eventHelpHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventHelpHandler] %s", clientId)

	var events = make([]string, 0)
	for event, _ := range x.eventHandlers {
		events = append(events, event)
	}

	var userEvents = make([]string, 0)
	for event, _ := range x.userEventHandlers {
		userEvents = append(userEvents, event)
	}

	x.Send(clientId, websocket.TextMessage, x.ToBytes(H{
		"baseEvents": events,
		"userEvents": userEvents,
		"clientId":   clientId,
	}))
	return true
}

func (x *WebsocketManager) eventConnectHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventConnectHandler] %s", clientId)

	x.Conn.Store(clientId, ws)

	x.Send(clientId, websocket.TextMessage, x.ToBytes(EventProtocolConnect{
		ClientId: clientId,
		Event:    Event(EventConnect).String(),
		Data: struct {
			ClientId string `json:"client_id"`
		}(struct{ ClientId string }{
			ClientId: clientId,
		}),
	}))

	return true
}

func (x *WebsocketManager) eventCloseHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventCloseHandler] %s, %d", clientId, messageType)

	x.Conn.Remove(clientId)
	// TODO 删除Uid数据，删除Group数据

	return true
}

func (x *WebsocketManager) eventStatHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventCloseHandler] %s, %d", clientId, messageType)

	var clientMap = make(map[string]map[string]interface{})
	for tmpClientId, tmpCtx := range x.Conn.Conn {
		clientMap[tmpClientId] = make(map[string]interface{})
		clientMap[tmpClientId]["Uid"] = tmpCtx.Uid
		clientMap[tmpClientId]["Group"] = tmpCtx.Group
		clientMap[tmpClientId]["Conn"] = tmpCtx.Conn.RemoteAddr().String()
	}

	var userMap = make(map[string][]string)
	for tmpUid, m := range x.Conn.Uid {
		userMap[tmpUid] = make([]string, 0)
		for tmpClientId, _ := range m {
			userMap[tmpUid] = append(userMap[tmpUid], tmpClientId)
		}
	}

	var groupMap = make(map[string][]string)
	for tmpGroup, m := range x.Conn.Group {
		groupMap[tmpGroup] = make([]string, 0)
		for tmpClientId, _ := range m {
			groupMap[tmpGroup] = append(groupMap[tmpGroup], tmpClientId)
		}
	}

	var respData = EventProtocol{
		ClientId: clientId,
		Event:    Event(EventStat).String(),
		Data: map[string]interface{}{
			"clientMap": clientMap,
			"userMap":   userMap,
			"groupMap":  groupMap,
		},
	}

	x.Send(clientId, websocket.TextMessage, x.ToBytes(respData))

	return true
}

func (x *WebsocketManager) eventBindUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventBindUidHandler] %s", clientId)

	var tmpUid = ""
	switch data.Data.(type) {
	case string:
		tmpUid = data.Data.(string)
	case float64:
		tmpUid = fmt.Sprintf("%f", data.Data.(float64))
	case float32:
		tmpUid = fmt.Sprintf("%f", data.Data.(float32))
	}

	if len(tmpUid) <= 0 {
		return false
	}
	x.Conn.SetUid(clientId, tmpUid)

	return true
}

func (x *WebsocketManager) eventPingHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventPingHandler] %s, %d", clientId, messageType)
	return true
}

func (x *WebsocketManager) eventSendToClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventSendToClientHandler] %s", clientId)

	switch data.Data.(type) {
	case string:
		x.Send(data.Data.(string), messageType, x.ToBytes(data))
		return true
	}

	return true
}

func (x *WebsocketManager) eventSendToUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventSendToUidHandler] %s", clientId)

	var clients []string
	switch data.Data.(type) {
	case string:
		clients = x.Conn.GetUidClientId(data.Data.(string))
	default:
		return false
	}
	var transferBuffer = x.ToBytes(data)
	for _, tmpClientId := range clients {
		x.Send(tmpClientId, messageType, transferBuffer)
	}
	return true
}

func (x *WebsocketManager) eventSendToGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventSendToGroupHandler] %s", clientId)

	if len(data.Group) == 0 {
		x.Log("[SendToGroup] missing param(group)")
		return false
	}
	var clients = x.Conn.GetGroupClientIds(data.Group)
	if len(clients) == 0 {
		return true
	}
	var transferBuffer = x.ToBytes(data)
	for _, tmpClientId := range clients {
		if clientId == tmpClientId {
			continue
		}
		x.Send(tmpClientId, messageType, transferBuffer)
	}
	return true
}

func (x *WebsocketManager) eventBroadcastHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventBroadcastHandler] %s", clientId)
	for tmpClientId, _ := range x.Conn.Conn {
		x.Send(tmpClientId, messageType, x.ToBytes(data))
	}
	return true
}

func (x *WebsocketManager) eventJoinGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventJoinGroupHandler] %s", clientId)

	if len(data.Group) == 0 {
		x.Log("[SendToGroup] missing param(group)")
		return false
	}
	x.Conn.JoinGroup(clientId, data.Group)
	return true
}

func (x *WebsocketManager) eventLeaveGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventLeaveGroupHandler] %s", clientId)

	if len(data.Group) == 0 {
		x.Log("[SendToGroup] missing param(group)")
		return false
	}
	x.Conn.LeaveGroup(clientId, data.Group)
	return true
}

func (x *WebsocketManager) eventListGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventListGroupHandler] %s", clientId)

	x.Send(clientId, messageType, x.ToBytes(H{
		"groups": x.Conn.ListGroup(),
	}))

	return true
}

func (x *WebsocketManager) eventListGroupClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	x.Log("[eventListGroupClientHandler] %s", clientId)

	if len(data.Group) == 0 {
		x.Log("[SendToGroup] missing param(group)")
		return false
	}

	x.Send(clientId, messageType, x.ToBytes(H{
		"groups": x.Conn.ListGroupClientIds(data.Group),
	}))
	return true

}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, messageType int, data []byte) bool {
	var conn = x.Conn.LoadConn(clientId)
	if conn == nil {
		return false
	}
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return false
	}
	if err := conn.WriteMessage(messageType, data); err != nil {
		return false
	}
	return true
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, messageType int, data []byte) bool {
	var clientIds = x.Conn.ListGroupClientIds(groupName)
	for _, clientId := range clientIds {
		x.Send(clientId, messageType, data)
	}
	return true
}
