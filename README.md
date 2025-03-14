# demo启动方式

```code
go run _examples/main.go
```

浏览器打开[http://127.0.0.1:8088](http://127.0.0.1:8088)

# 使用方式

### 说明
- 基于`github.com/gorilla/websocket`包

### 导入包
```code
go get github.com/lixiang4u/goWebsocket
```

### 实例化对象
```code
import (
	"github.com/lixiang4u/goWebsocket"
)

var ws = goWebsocket.NewWebsocketManager()
```

### 注册响应事件

- 内置事件返回true才会执行自定义事件，否则直接执行自定义事件

```code
ws.On(eventName string, f eventHandler)
```

- 需要客户端请求数据格式为`protocol`对象的json字面量
```code
type EventProtocol struct {
	Event    string      `json:"event"`
	Data     interface{} `json:"data"`
}
```

- 内置事件如下（部分事件不直接对外暴露）：
```code
"connect",
"sendToUid",
"listGroup",
"listGroupClient",
"joinGroup",
"leaveGroup",
"close",
"ping",
"bindUid",
"sendToClient",
"sendToGroup"
```

### 运行
http
```code
ws.Handler(w http.ResponseWriter, r *http.Request, responseHeader http.Header)
```

gofiber v3
```code
// import "github.com/gofiber/fiber/v3"
// import "github.com/gofiber/fiber/v3/middleware/adaptor"
// import "github.com/lixiang4u/goWebsocket"

var socket = goWebsocket.NewWebsocketManager()
app := fiber.New()
app.Get("/websocket", adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
    socket.Handler(writer, request, nil)
}))
```

### 广播聊天截图


![markdown](https://raw.githubusercontent.com/lixiang4u/go-websocket/master/_examples/screenshot.png "markdown")
