package go_websocket

import (
	"fmt"
	"github.com/gorilla/websocket"
	"strings"
)

const (
	EventHelp    = iota
	EventConnect = iota
	EventClose   = iota
	EventStat    = iota

	EventPing    = iota
	EventBindUid = iota

	EventSendToClient = iota
	EventSendToUid    = iota
	EventSendToGroup  = iota
	EventBroadcast    = iota

	EventJoinGroup       = iota
	EventLeaveGroup      = iota
	EventListGroup       = iota
	EventListGroupClient = iota
)

type Event int

// EventHandler 事件响应格式
type EventHandler func(clientId string, ws *websocket.Conn, messageType int, data EventProtocol) bool

func (x Event) String() string {
	var eventName = ""
	switch x {
	case EventHelp:
		eventName = "Help"
	case EventConnect:
		eventName = "Connect"
	case EventClose:
		eventName = "Close"
	case EventStat:
		eventName = "Stat"
	case EventPing:
		eventName = "Ping"
	case EventBindUid:
		eventName = "BindUid"
	case EventSendToClient:
		eventName = "SendToClient"
	case EventSendToUid:
		eventName = "SendToUid"
	case EventSendToGroup:
		eventName = "SendToGroup"
	case EventBroadcast:
		eventName = "Broadcast"
	case EventJoinGroup:
		eventName = "JoinGroup"
	case EventLeaveGroup:
		eventName = "LeaveGroup"
	case EventListGroup:
		eventName = "ListGroup"
	case EventListGroupClient:
		eventName = "ListGroupClient"
	default:
		panic("错误事件")
	}
	if len(eventName) > 0 {
		var r = []rune(eventName)
		eventName = fmt.Sprintf("%s%s", strings.ToLower(string(r[0])), string(r[1:len(r)]))
	}
	return eventName
}
