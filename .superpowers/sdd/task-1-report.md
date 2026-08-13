# Task 1 报告：配置与安全日志

状态：`DONE_WITH_CONCERNS`

## 实现文件

- 新增 `config.go`：`ManagerConfig`、默认值与非正值归一化、新配置构造器，以及兼容旧 `NewWebsocketManager(debug ...bool)` 的包装器。
- 新增 `config_test.go`：配置默认值、归一化、旧构造器兼容性、通道无缓冲兼容性、实例读限制、默认负载脱敏、显式负载日志开关和 `Debug=false` 的端到端 WebSocket 测试。
- 修改 `websocket.go`：保存 `ManagerConfig`、按实例设置读限制；入站与协议错误日志默认仅记录类型/字节数，只有 `LogPayload=true` 才输出原始入站负载。
- 修改 `_examples/socket.go`：所有 `log.Print*` 调用均改为单一固定事件类别，不记录 `EventCtx`、uid、group、client ID、客户端快照或接收者列表。

## TDD 证据

### RED

命令：`go test ./... -run 'ManagerConfig|PayloadLogging' -count=1`

结果：退出码 `1`，预期失败。编译器报告 `NewWebsocketManagerWithConfig` 和 `ManagerConfig` 未定义，并报告旧匿名 `Config` 缺少 `LogPayload`、`ReadLimitBytes`、`OutboundQueueSize`、`EventQueueSize` 字段。失败原因仅为待实现的公开接口缺失。

### GREEN

命令：`gofmt -w config.go config_test.go websocket.go`，随后运行 `go test ./... -run 'ManagerConfig|PayloadLogging' -count=1`。

结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 1.518s`。

### 竞态检测

命令：`go test -race ./... -run 'ManagerConfig|PayloadLogging' -count=1`

结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 2.647s`。

首次在受限 Go 构建缓存中运行 race 测试时因 `go-build` 缓存权限失败；随后仅授予构建缓存访问权限重跑，仓库文件未被该授权修改。

### 审查修复 RED/GREEN

RED 命令：`go test ./... -run 'ManagerConfig|PayloadLogging|ExampleSocket' -count=1`

RED 结果：退出码 `1`。`TestManagerConfigDefaults` 报告十个既有通道 cap 均为 `64` 而非无缓冲；`TestExampleSocketDoesNotLogRawEventContext` 报告 `_examples/socket.go` 仍记录 `JsonString(ctx)`。

GREEN 命令：`gofmt -w config.go config_test.go websocket.go _examples/socket.go`，随后运行 `go test ./... -run 'ManagerConfig|PayloadLogging|ExampleSocket' -count=1`。

GREEN 结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 1.492s`。

审查修复 race 命令：`go test -race ./... -run 'ManagerConfig|PayloadLogging|ExampleSocket' -count=1`

审查修复 race 结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 2.631s`。

### 入站派生日志复审 RED/GREEN

RED 命令：`go test ./... -run 'ExampleSocket' -count=1`

RED 结果：退出码 `1`。AST 静态测试发现 11 个动态 `log.Print*` 参数，分别来自 `ctx.From`、`err.Error()`、`req.GroupName`、`AppSocket.ListConn()` 和 `AppSocket.ListGroupClient(...)`；这些表达式可携带入站派生的 uid、group 或 client ID。

GREEN 命令：`gofmt -w config_test.go _examples/socket.go`，随后运行 `go test ./... -run 'ExampleSocket' -count=1`。

GREEN 结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 1.483s`。

最终聚焦命令：`go test ./... -run 'ManagerConfig|PayloadLogging|ExampleSocket' -count=1`

最终聚焦结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 1.485s`。

最终 race 命令：`go test -race ./... -run 'ManagerConfig|PayloadLogging|ExampleSocket' -count=1`

最终 race 结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 2.647s`。

## 审计

- 兼容性：旧可变参数构造器仍存在，仅映射第一个布尔参数到 `Debug`；`LogPayload` 固定为默认 `false`，其余配置使用默认值。
- 安全性：`Debug=false` 时 `Log` 不输出；`Debug=true && LogPayload=false` 的正常入站日志仅含消息类型和字节数，协议错误仅含固定分类与字节数；测试确认 JSON、字段名和字段值均不会泄露。`LogPayload=true` 时才会记录原始入站负载。`LogForce` 未修改。
- 示例安全性：AST 回归测试要求 `_examples/socket.go` 的每一个 `log.Print*` 都只能接收一个静态字符串字面量；因此 ctx/req/AppSocket 派生的 uid、group、client ID、连接快照和接收者列表均无法流入默认示例日志。
- 队列与读限制：`EventQueueSize` 和 `OutboundQueueSize` 在 `ManagerConfig` 中按默认值 64 归一化并保留，供后续 per-client 队列任务使用；既有 manager 的十个事件/出站通道保持无缓冲，未改变原有 backpressure。`readMessage` 使用归一化后的实例 `ReadLimitBytes`；真实 WebSocket 测试证明 64 字节限制会关闭超限连接，256 字节限制可接收同一帧，默认值为 64 KiB。
- 保留项：未触碰现有写心跳/ticker 区域及其注释；未执行 add、commit 或 push。
- 依赖文件：`go.mod`/`go.sum` 没有被本任务修改。执行前后的预存状态均为索引中 `D  go.mod`，工作区中未跟踪 `go.mod` 和 `go.sum`。
- 格式：`gofmt` 已执行；`git diff --check` 对已跟踪差异无空白错误。

## Concerns

- 当前仓库在任务开始前已有异常依赖状态：索引删除了 `go.mod`，但工作区存在未跟踪的替代 `go.mod/go.sum`。本任务严格保留它；测试实际使用工作区的未跟踪模块文件。后续任务应先由拥有者恢复或确认该 Git 索引状态。
- `git diff --check` 不会覆盖未跟踪的新文件；新 Go 文件已由 `gofmt` 格式化，且 GREEN/race 测试均已编译并通过。
- `WebsocketManager` 的 `registerEvent` 后台 goroutine 目前没有停止机制；在 Task 5 引入 `Close` 前，这是已知的生命周期限制。本任务按要求保留既有后台循环。
