package go_websocket

// ws数据交互格式，基于json，event字段必选
type protocol struct {
	ClientId string      `json:"client_id"`
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}
