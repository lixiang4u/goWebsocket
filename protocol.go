package goWebsocket

// EventProtocol ws数据交互格式，基于json，event字段必选
type EventProtocol struct {
	ClientId string      `json:"client_id,omitempty"` // 调用sendToClient时的client
	Group    string      `json:"group,omitempty"`     // 调用sendToGroup时的group
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
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
	FromId string      `json:"from_id"`
	ToId   string      `json:"to_id"`
	Event  string      `json:"event"`
	Data   interface{} `json:"data"`
}
