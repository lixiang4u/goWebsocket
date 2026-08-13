# goWebsocket 有序事件与有界发送队列设计

## 1. 目标与兼容性

在保持 `NewWebsocketManager`、`On`、`Send`、`SendToGroup`、`SendToUid`、`SendToAll`、`JoinGroup`、`LeaveGroup` 现有签名和基础语义的前提下，增加同连接事件 FIFO、每连接单 writer、有界发送队列、精确群组入队结果、安全日志和可关闭生命周期。

AirPlayTV 是首个使用精确 ACK 语义的调用方，但实现必须保持通用，不包含 AirPlayTV 业务事件。

## 2. 同连接 FIFO 用户事件

当前 `dispatchUserEvent()` 为每个回调启动独立 goroutine，同一连接的 Presence、切房和 Close 会乱序。改为按 `EventCtx.From` 分区的串行执行器：

- 每个连接一个有界 `chan userEventTask`；
- 同一连接的 Connect、入站业务事件和 Close 按提交顺序执行；
- 不同连接使用不同 worker 并发处理；
- Close 排在此前已读取帧之后，完成后销毁连接事件队列；
- 无 `From` 的内部事件使用独立全局队列；
- 队列满时不阻塞全局循环，关闭异常连接并记录固定错误类别；
- handler panic 由 worker 恢复，不能终止 worker。

连接事件必须在注册完成后提交。Close 必须通过 terminal task 保证业务 Close handler 获得断开上下文，且不会被迟到任务重新建立业务状态。

## 3. 每连接有界发送队列

每个连接维护唯一 writer：

```go
type connectionWriter struct {
    socket *websocket.Conn
    queue  chan outboundFrame
    done   chan struct{}
}
```

- 只有 writer goroutine 调用 Gorilla WebSocket 写方法；
- 业务消息和 Ping 均由同一 writer 串行写出；
- 默认队列容量 64，可配置；
- 入队非阻塞，队列满返回拒绝并触发连接关闭/清理；
- 单帧写超时默认 45 秒，但只阻塞该连接；
- 写错误触发幂等断开；
- manager 全局循环不得执行真实网络写入；
- disconnect/Close 不能重复关闭 channel 或向已关闭 channel 发送。

## 4. 精确入队接口

新增：

```go
type EnqueueResult struct {
    RecipientIDs []string
    Accepted     int
    Rejected     int
}

func (x *WebsocketManager) EnqueueToGroup(group string, data interface{}) EnqueueResult
```

- 一次调用取得 group 客户端 ID 快照；
- 对快照逐个尝试写入对应连接 queue；
- `Accepted` 为成功入队数量；`Rejected` 为不存在、关闭中或队列满数量；
- `RecipientIDs` 仅包含成功入队的 ID，排序后返回副本；
- 返回仅保证已入队，不保证网络写出或客户端处理；
- 空 group 或编码失败返回零值。

旧 `SendToGroup` 复用相同入队逻辑但保持无返回值。

## 5. Handler 注册安全

`On()` 与 handler 读取使用独立 `sync.RWMutex`：锁内只读取函数值，释放锁后执行。保持同事件后注册覆盖前注册的语义。

## 6. 安全日志与配置

新增命名配置，同时保留旧 bool 构造：

```go
type ManagerConfig struct {
    Debug             bool
    LogPayload        bool
    ReadLimitBytes    int64
    OutboundQueueSize int
    EventQueueSize    int
}
```

- `NewWebsocketManager(debug ...bool)` 的 bool 只映射 `Debug`；`LogPayload` 默认 false；
- 新增 `NewWebsocketManagerWithConfig(config ManagerConfig)`；
- Debug 默认只记录固定事件、计数和错误类别；
- 只有显式 `LogPayload=true` 才打印完整入站 payload；
- 协议错误在 `LogPayload=false` 时不得打印原文；
- 默认 ReadLimit 64 KiB；非正值回退默认值。

## 7. 生命周期

新增幂等：

```go
func (x *WebsocketManager) Close() error
```

- 停止 manager loop；
- 关闭所有连接；
- 等待事件 worker 和 writer 退出；
- Close 后新发送不得 panic；
- 重复 Close 安全；
- 生产调用方可不显式调用，测试必须调用以防 goroutine 泄漏。

## 8. 并发不变量

- 同一连接用户事件严格 FIFO，不同连接可并行。
- 同一 socket 永远只有一个 writer。
- group snapshot 和连接入队结果属于同一次 `EnqueueToGroup`。
- 慢客户端只能阻塞自身 writer，不能阻塞其他连接、ACK 或 manager loop。
- 连接删除、writer 退出和 Close handler 幂等。
- handler map、worker map、writer 和公共索引在 race detector 下无竞争。

## 9. 测试

- 同连接 A→B→Close FIFO；不同连接并行；handler panic 隔离。
- 事件队列满的有界行为。
- 群组入队 accepted/rejected/recipient IDs。
- 阻塞 writer 不影响其他连接入队及调用返回。
- 业务消息与 Ping 单 writer，无 concurrent write。
- outbound queue 满、写失败、disconnect 和重复 Close 清理。
- Debug=true + LogPayload=false 不含原始 payload；显式 true 才包含。
- 默认/自定义 read limit。
- `go test ./...` 与 `go test -race ./...`。

## 10. AirPlayTV 接入

公共库验证通过后，AirPlayTV API：

- 使用安全配置关闭 payload 日志；
- Gateway 使用 `EnqueueToGroup`，以 `Accepted` 生成 ACK；
- Presence/Close 依赖同连接 FIFO；
- Controller 继续负责 Registry、租约及 pair/unpair；
- 不复制公共库实现。

开发期使用 `go.work` 或临时 `go.mod replace` 指向本地库。正式部署必须发布公共库新版本，或由用户明确允许保留生产 replace。

## 11. 工作区与 Git 边界

- 保留公共库现有 `websocket.go` 日志注释改动。
- 不覆盖当前 `go.mod` 删除/重建混合状态及其他无关未跟踪文件。
- 不执行 `git add`、`git commit` 或 `git push`。
- 所有新增和修改文件使用 UTF-8 无 BOM。
