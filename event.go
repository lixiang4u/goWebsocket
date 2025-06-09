package goWebsocket

const (
	EventHelp    = iota
	EventConnect = iota
	EventClose   = iota
	EventStat    = iota

	EventPing      = iota
	EventBindUid   = iota
	EventUnbindUid = iota

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
type EventHandler func(data EventCtx) bool

func (x Event) String() string {
	var eventName = ""
	switch x {
	case EventHelp:
		eventName = "x-app-event-help"
	case EventConnect:
		eventName = "x-app-event-connect"
	case EventClose:
		eventName = "x-app-event-close"
	case EventStat:
		eventName = "x-app-event-stat"
	case EventPing:
		eventName = "x-app-event-ping"
	case EventBindUid:
		eventName = "x-app-event-bind-uid"
	case EventUnbindUid:
		eventName = "x-app-event-unbind-uid"
	case EventSendToClient:
		eventName = "x-app-event-send-to-client"
	case EventSendToUid:
		eventName = "x-app-event-send-to-uid"
	case EventSendToGroup:
		eventName = "x-app-event-send-to-group"
	case EventBroadcast:
		eventName = "x-app-event-broadcast"
	case EventJoinGroup:
		eventName = "x-app-event-Join-Group"
	case EventLeaveGroup:
		eventName = "x-app-event-leave-group"
	case EventListGroup:
		eventName = "x-app-event-list-group"
	case EventListGroupClient:
		eventName = "x-app-event-list-group-client"
	default:
		panic("错误事件")
	}
	return eventName
}
