# Task 1 变更摘要

## `config.go`（新增）

- 定义公开的 `ManagerConfig`：`Debug`、`LogPayload`、`ReadLimitBytes`、`OutboundQueueSize`、`EventQueueSize`。
- 定义默认值：读限制 `64 << 10`，两个队列容量均为 `64`；所有非正数值在构造时归一化为默认值。
- 新增 `NewWebsocketManagerWithConfig(config ManagerConfig)`：初始化既有 clients/users/groups 和既有后台 `registerEvent` 循环；十个既有 manager 通道保持无缓冲，队列大小配置仅归一化并保存，留给后续 per-client 队列任务使用。
- 将旧 `NewWebsocketManager(debug ...bool)` 改为兼容包装器，仅传递 `Debug`。

## `websocket.go`（修改）

- `WebsocketManager.Config` 从仅含 `Debug` 的匿名结构体改为 `ManagerConfig`。
- `readMessage` 的 Gorilla WebSocket 读限制改为 `x.Config.ReadLimitBytes`。
- 正常入站日志改由 `logInboundMessage` 处理：默认输出消息类型和字节数；仅在 `LogPayload=true` 时输出原始 JSON。
- 协议错误日志改由 `logProtocolError` 处理：默认输出固定 `[WebsocketRequestProtocolError]` 分类和字节数；仅在 `LogPayload=true` 时输出原始负载。
- `Log` 和 `LogForce` 的既有语义保持不变；写心跳/ticker 区域未改动。

## `_examples/socket.go`（修改）

- 所有 `log.Print*` 调用改为单一固定 `event=<类别>` 字符串；不再输出 `EventCtx.Data`、`ctx.From`、解码错误、uid、group、client ID、连接快照或接收者列表。

## `config_test.go`（新增）

- 覆盖默认配置和全部队列容量。
- 覆盖非正读限制/队列容量归一化。
- 覆盖旧构造器仅映射 `Debug`。
- 覆盖十个既有 manager 通道保持无缓冲，以及 `ReadLimitBytes` 按 manager 实例生效：超限帧关闭小限制连接，同一帧由大限制连接接收。
- 使用 `httptest` 和真实 Gorilla WebSocket 客户端覆盖默认脱敏、显式日志开关和关闭 Debug 的行为。
- AST 静态回归测试确保 `_examples/socket.go` 的每个 `log.Print*` 都只有一个字符串字面量参数，禁止任何 ctx/req/AppSocket 派生标识进入日志。
- 通过带互斥锁的缓冲区捕获全局 logger，并用 `t.Cleanup` 恢复原始 logger；未使用并行测试。
