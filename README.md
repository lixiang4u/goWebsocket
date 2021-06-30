# 使用方式

### 导入包
```go
go get github.com/lixiang4u/go-websocket
```

### 实例化对象
```go
var ws = go_websocket.WSWrapper{}
```

### 注册响应事件
```go
ws.On(eventName string, f eventHandler)
```

- 需要客户端请求数据格式为`protocol`对象的json字面量
```go
type protocol struct {
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}
```

- 测试返回的数据格式为`protocol`对象的json字面量
```go
type protocol struct {
	ClientId string      `json:"client_id"`
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}
```


### 运行
```go
ws.Run(w http.ResponseWriter, r *http.Request, responseHeader http.Header)
```

### 广播聊天截图


![markdown](https://raw.githubusercontent.com/lixiang4u/go-websocket/master/_examples/screenshot.png "markdown")
