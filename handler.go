package goWebsocket

import (
	"github.com/gorilla/websocket"
)

func (x *WebsocketManager) eventHelpHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventConnectHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventCloseHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventStatHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventBindUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventPingHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventBroadcastHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventJoinGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventLeaveGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventListGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventListGroupClientHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true

}

// Send 对外接口，用于发送ws消息到指定clientId
func (x *WebsocketManager) Send(clientId string, messageType int, data []byte) bool {
	//var conn = x.data.LoadConn(clientId)
	//if conn == nil {
	//	return false
	//}
	//if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
	//	return false
	//}
	//if err := conn.WriteMessage(messageType, data); err != nil {
	//	return false
	//}
	return true
}

// SendToGroup 发送消息到组
func (x *WebsocketManager) SendToGroup(groupName string, messageType int, data []byte) bool {
	return true
}

func (x *WebsocketManager) SendToUid(uid string, messageType int, data []byte) bool {
	return true
}
