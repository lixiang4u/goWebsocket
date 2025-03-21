package goWebsocket

import "github.com/gorilla/websocket"

type ConnectionCtx struct {
	Socket *websocket.Conn `json:"socket,omitempty"`
	Group  map[string]bool `json:"group"`
	Uid    string          `json:"uid"`
}

// EventCtx 消息交换格式
type EventCtx struct {
	From    string          `json:"from,omitempty"`     // 来源客户端ID，可能为空
	ToId    string          `json:"to_id,omitempty"`    // 接收消息的客户端Id；发送人不一定知道（可能直接方法调用 appSocket.SendToGroup）
	ToGroup string          `json:"to_group,omitempty"` // 接收消息的组名/ID
	ToUid   string          `json:"to_uid,omitempty"`   // 接收消息的用户ID
	Socket  *websocket.Conn `json:"socket,omitempty"`   // 发送消息的连接
	Data    interface{}     `json:"data"`               // 数据
	Event   string          `json:"event,omitempty"`    // 【只有通过websocket发送消息才有的事件】websocket 事件名,通过websocket直接通信使用；ws数据交互格式 基于json event字段必选
}
