package goWebsocket

import "github.com/gorilla/websocket"

type ConnectionCtx struct {
	Socket *websocket.Conn `json:"socket,omitempty"`
	Group  map[string]bool `json:"group"`
	Uid    string          `json:"uid"`
}

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

// EventCtx 消息交换格式
type EventCtx struct {
	Id     string          `json:"id"`               // 客户端Id
	Group  string          `json:"group,omitempty"`  // 组名/ID
	Uid    string          `json:"uid,omitempty"`    // 用户ID
	Socket *websocket.Conn `json:"socket,omitempty"` // 连接
	Data   interface{}     `json:"data"`             // 数据
	Event  string          `json:"event,omitempty"`  // websocket 事件名,通过websocket直接通信使用；ws数据交互格式 基于json event字段必选
}
