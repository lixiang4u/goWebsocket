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
	x.Send(data.ClientId, ToBuff(data.Data))
	return true
}

func (x *WebsocketManager) eventSendToUidHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventSendToGroupHandler(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool {
	return true
}

func (x *WebsocketManager) eventBroadcastHandler(clientId string, ws *websocket.Conn, messageType int, data MessageProtocol) bool {
	x.send <- data
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
