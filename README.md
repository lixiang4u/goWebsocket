# goWebsocket

基于 Gorilla WebSocket 的连接、用户和分组管理器。同一连接的入站事件严格 FIFO；每个连接拥有独立、有界的单 writer 队列。

## 安装

```bash
go get github.com/lixiang4u/goWebsocket
```

## 配置

旧构造器继续可用，其可选布尔参数只控制 `Debug`：

```go
manager := goWebsocket.NewWebsocketManager(true)
```

新代码建议显式配置：

```go
manager := goWebsocket.NewWebsocketManagerWithConfig(goWebsocket.ManagerConfig{
	Debug:             true,
	LogPayload:        false,
	ReadLimitBytes:    64 << 10,
	OutboundQueueSize: 64,
	EventQueueSize:    64,
})
defer manager.Close()
```

数值字段小于等于零时使用默认值：

| 配置 | 默认值 | 语义 |
| --- | ---: | --- |
| `ReadLimitBytes` | 64 KiB | 单个入站消息的 Gorilla read limit |
| `OutboundQueueSize` | 64 | 每连接文本帧和 Ping 共用的发送队列容量 |
| `EventQueueSize` | 64 | 每连接业务事件 FIFO 队列容量 |

`Debug=true` 默认只记录固定事件分类、消息类型和字节数。只有显式设置 `LogPayload=true` 才会记录入站原文；生产环境不应开启该选项。

连接事件队列第一次溢出时仅记录固定 `[WebsocketEventQueueFull]` 分类；同一连接实例只记录一次，不包含 client ID、uid、group、payload 或底层错误。

## HTTP 接入

```go
http.HandleFunc("/websocket", func(w http.ResponseWriter, r *http.Request) {
	manager.Handler(w, r, nil)
})
```

客户端业务帧使用 JSON：

```json
{
  "event": "video-control",
  "data": {}
}
```

注册事件：

```go
manager.On("video-control", func(ctx goWebsocket.EventCtx) bool {
	return true
})
```

同一连接的 `Connect -> 入站业务事件 -> Close` 按接受顺序执行；不同连接的 handler 可以并行。handler panic 会被隔离，不中断同连接后续事件。

## 分组入队结果

需要可观测投递边界时使用 `EnqueueToGroup`：

```go
result := manager.EnqueueToGroup("room-123", payload)
```

- `Accepted`：调用返回前，帧已进入对应连接的 outbound queue。
- `Rejected`：快照中的连接已关闭、已被替换、队列已满或 identity 校验失败。
- 两个列表均按 client ID 排序。
- `Accepted` 不表示已经写入网络，更不表示客户端已接收或处理。端到端确认必须由应用协议携带 `request_id` 并返回 ACK。

旧 `Send`、`SendToGroup`、`SendToUid`、`SendToAll` 签名保持兼容，内部同样使用连接级队列。

## 关闭语义

```go
if err := manager.Close(); err != nil {
	log.Fatal(err)
}
```

`Close` 幂等且并发安全。它原子拒绝新的连接、绑定、分组和发送操作，停止 register loop，关闭当前连接 transport，停止 writer/heartbeat，并只等待 manager core、已登记 public operation、read loop、writer 和 heartbeat 退出。关闭后的 void API 快速 no-op；`EnqueueToGroup` 返回空的 `Accepted` 和 `Rejected`。

Go 无法强制终止正在执行的用户 handler。为避免 Close handler 调用 `Close`、`Send` 或 `Disconnect` 时自锁，`Close` 不等待 FIFO/terminal 用户 handler 或旧发送 API 的 callback goroutine。已经接受的 FIFO 事件仍按顺序 drain，terminal Close 恰好提交一次，worker 在 handler 返回后清理。应用必须自行保证 handler 最终返回；需要取消时，应在业务 payload 或应用拥有的 context 中传递取消信号。

旧发送 API 的用户 callback 保持历史行为：每次 callback 在独立 goroutine 执行，manager `Close` 不等待它们。长期阻塞的 callback 可能持续占用 goroutine。

## 示例

```bash
go run _examples/main.go
```

打开 [http://127.0.0.1:8088](http://127.0.0.1:8088)。
