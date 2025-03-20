package goWebsocket

// EventProtocol ws数据交互格式，基于json，event字段必选
type EventProtocol struct {
	ClientId string      `json:"client_id,omitempty"` // 发送目标客户端ID
	Group    string      `json:"group,omitempty"`     // 发送目标客户端组名称
	Event    string      `json:"event"`               // websocket 事件名,目前支持内置的 SendToClient, SendToGroup两个; 通过 registerEvents 注册
	Data     interface{} `json:"data"`                // 对方接收的数据(json对象)
}

type EventProtocolConnect struct {
	ClientId string `json:"client_id,omitempty"`
	Event    string `json:"event"`
	Data     struct {
		ClientId string `json:"client_id"`
	} `json:"data"`
}

// MessageProtocol 消息类型
type MessageProtocol struct {
	FromId string      `json:"from_id,omitempty"`
	ToId   string      `json:"to_id,omitempty"`
	Data   interface{} `json:"data"`
}
