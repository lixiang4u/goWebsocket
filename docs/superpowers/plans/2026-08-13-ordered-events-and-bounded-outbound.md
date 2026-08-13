# goWebsocket 有序事件与有界发送队列实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不破坏现有公共 API 的前提下，使同连接事件严格 FIFO、网络写入按连接隔离，并为调用方提供精确的群组入队结果和安全日志配置。

**Architecture:** Manager 保留现有客户端、用户和群组索引，新增连接级 event worker 与 writer。所有 Gorilla WebSocket 写操作通过单 writer 的有界队列完成；`EnqueueToGroup` 对一次客户端快照返回实际入队结果。旧 fire-and-forget API 复用新队列但保持签名。

**Tech Stack:** Go 1.23.3、Gorilla WebSocket、concurrent-map、标准库 testing/httptest/sync/context。

## Global Constraints

- 保持现有公开方法签名和后注册 handler 覆盖旧 handler 的语义。
- 同一连接 Connect、入站业务事件、Close 严格 FIFO；不同连接可并行。
- 同一 Gorilla WebSocket 只能由一个 writer goroutine 写入。
- 默认 outbound queue=64、event queue=64、read limit=64 KiB、write timeout=45 秒。
- 队列操作必须有界且非阻塞；队列满关闭异常连接，不得阻塞其他连接。
- `EnqueueToGroup` 的 Accepted 仅表示进入对应连接队列，不表示网络写出或客户端处理。
- Debug 默认不得记录 payload；只有显式 `LogPayload=true` 才可记录。
- 保留现有 `websocket.go` 日志注释及 `go.mod` 删除/重建混合状态，不修改 `go.mod`/`go.sum`。
- 保留所有无关脏文件；不执行 git add、commit、push。
- 所有新增和修改文件使用 UTF-8 无 BOM。

---

## 文件结构

- `config.go`：`ManagerConfig` 默认值和规范化。
- `event_dispatcher.go`：按 client ID 的有界 FIFO worker。
- `writer.go`：连接级 outbound queue 与唯一 writer。
- `websocket.go`：Manager 生命周期、Handler、read loop 和注册循环编排。
- `connection.go`：连接索引与 writer 创建/清理。
- `handler.go`：旧发送 API 适配和 `EnqueueToGroup`。
- `protocol.go`：必要的内部/公开结果类型。
- `*_test.go`：事件顺序、发送隔离、配置、日志和生命周期测试。

---

### Task 1: 配置与安全日志

**Files:**
- Create: `config.go`
- Create: `config_test.go`
- Modify: `websocket.go`

**Interfaces:**
- Produces: `ManagerConfig`。
- Produces: `NewWebsocketManagerWithConfig(config ManagerConfig) *WebsocketManager`。
- Preserves: `NewWebsocketManager(debug ...bool) *WebsocketManager`。

- [ ] **Step 1: 写配置和日志 RED 测试**

验证默认值、非正值回退、旧 bool 只控制 Debug、`Debug=true/LogPayload=false` 不记录敏感 payload、显式 true 才记录 payload。日志测试临时替换标准 logger output，并在结束时恢复。

- [ ] **Step 2: 运行 RED**

Run: `go test ./... -run 'ManagerConfig|PayloadLogging' -count=1`

Expected: 编译失败，缺少新配置。

- [ ] **Step 3: 实现配置规范化**

```go
const (
    defaultReadLimitBytes    int64 = 64 << 10
    defaultOutboundQueueSize       = 64
    defaultEventQueueSize          = 64
)
```

Manager 保存规范化配置。入站成功日志只有在 `LogPayload` 为 true 时附带正文；协议错误默认仅记录固定 `WebsocketRequestProtocolError` 类别。

- [ ] **Step 4: 运行 GREEN**

Run: `gofmt -w config.go config_test.go websocket.go`

Run: `go test ./... -run 'ManagerConfig|PayloadLogging' -count=1`

Expected: PASS。

---

### Task 2: 同连接 FIFO 事件执行器

**Files:**
- Create: `event_dispatcher.go`
- Create: `event_dispatcher_test.go`
- Modify: `websocket.go`

**Interfaces:**
- Consumes: `ManagerConfig.EventQueueSize`。
- Produces internal: `submitClientEvent(clientID string, task userEventTask) bool`。
- Produces internal terminal task for Close。

- [ ] **Step 1: 写 FIFO RED 测试**

覆盖同 client A→B→Close 顺序、两个 client 并行、handler panic 后同 client 后续事件继续、terminal 后拒绝新任务、队列满快速返回。使用 channels/barriers 控制调度，禁止依赖 sleep 推断顺序。

- [ ] **Step 2: 运行 RED**

Run: `go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

- [ ] **Step 3: 实现有界 per-client worker**

worker map 由 mutex 保护；task 在锁外执行。terminal task 完成后 worker 删除自己，删除必须验证实例 identity，防止旧 worker 删除同 ID 的新 worker。

`dispatchUserEvent` 获取 handler 时使用 handler RWMutex，随后向 client worker 提交；无 From 的内部回调使用 global worker。

- [ ] **Step 4: 调整 read/Close 提交顺序**

入站帧按 read loop 顺序提交；读错误提交 terminal Close task。确保业务 Close handler 执行后再做最终 worker 清理，且新的同 ID 连接不受旧 terminal 影响。

- [ ] **Step 5: GREEN 与 race**

Run: `gofmt -w event_dispatcher.go event_dispatcher_test.go websocket.go`

Run: `go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

Run: `go test -race ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

Expected: PASS，无 race。

---

### Task 3: 每连接单 writer 与有界 outbound queue

**Files:**
- Create: `writer.go`
- Create: `writer_test.go`
- Modify: `connection.go`
- Modify: `websocket.go`

**Interfaces:**
- Consumes: `ManagerConfig.OutboundQueueSize`、`writeWait`。
- Produces internal: `enqueueClient(clientID string, frame outboundFrame) bool`。

- [ ] **Step 1: 写 writer RED 测试**

通过可注入 socket writer interface 覆盖：消息顺序、Ping/文本同 writer、一个阻塞 writer 不影响另一连接入队、queue full 返回 false、写失败只清理本连接、disconnect/重复 close 无 panic。

- [ ] **Step 2: 运行 RED**

Run: `go test ./... -run 'ConnectionWriter|OutboundQueue' -count=1`

- [ ] **Step 3: 实现 writer**

writer queue 保存预编码 bytes 与 message type。writer 是唯一调用 SetWriteDeadline/WriteMessage 的 goroutine。Ping 通过 queue 投递；禁止保留旧 `writeMessage` 直接写 socket 的路径。

- [ ] **Step 4: 连接生命周期接入**

Connect 创建 writer 后才公开连接；disconnect 原子移除连接与 writer，并幂等 stop。写失败触发 manager disconnect，但不得由 writer 等待自身 WaitGroup。

- [ ] **Step 5: GREEN 与 race**

Run: `gofmt -w writer.go writer_test.go connection.go websocket.go`

Run: `go test ./... -run 'ConnectionWriter|OutboundQueue' -count=1`

Run: `go test -race ./... -run 'ConnectionWriter|OutboundQueue' -count=1`

Expected: PASS，无 concurrent-write/race。

---

### Task 4: `EnqueueToGroup` 与旧发送 API 兼容

**Files:**
- Modify: `protocol.go`
- Modify: `handler.go`
- Modify: `websocket.go`
- Create: `enqueue_test.go`

**Interfaces:**
- Produces: `EnqueueResult`。
- Produces: `EnqueueToGroup(group string, data interface{}) EnqueueResult`。

- [ ] **Step 1: 写群组入队 RED 测试**

验证空 group、编码失败、排序 recipient IDs、accepted/rejected、snapshot 中连接已关闭、queue full、慢客户端隔离。验证 `SendToGroup` 使用相同逻辑但无返回值，现有 send-to-group user callback 仍触发一次。

- [ ] **Step 2: 实现一次快照的入队**

先调用 `ToBuff` 一次；取得 group `Items()` 快照；按 client ID 排序后尝试 enqueue。结果 slice 为新分配副本。

- [ ] **Step 3: 旧 API 适配**

`Send`、`SendToGroup`、`SendToUid`、`SendToAll` 不再把网络写放入全局 manager loop；直接通过 thread-safe snapshot/enqueue helper，并保持对应 user event callback。

- [ ] **Step 4: GREEN 与兼容回归**

Run: `gofmt -w protocol.go handler.go websocket.go enqueue_test.go`

Run: `go test ./... -run 'Enqueue|SendToGroup|LegacySend' -count=1`

Run: `go test -race ./... -run 'Enqueue|SendToGroup|LegacySend' -count=1`

Expected: PASS。

---

### Task 5: Manager Close、真实 WebSocket 集成与全量验证

**Files:**
- Modify: `websocket.go`
- Modify: `handler.go`
- Create: `lifecycle_test.go`
- Create: `integration_test.go`
- Modify: `README.md`

- [ ] **Step 1: 写 Close RED 测试**

验证 Close 停止 manager、关闭连接、等待 worker/writer、重复 Close、Close 后 send 不 panic。使用 deadline 等待状态条件，不用固定长 sleep。

- [ ] **Step 2: 实现幂等 Close**

Manager 保存 context/cancel/WaitGroup/closed flag。所有后台 goroutine受统一生命周期控制；Close 不等待当前调用它的 worker 自身。

- [ ] **Step 3: 真实 transport 测试**

用 `httptest.Server` + Gorilla client 验证：同 socket A→B→Close FIFO；64 KiB 默认 read limit；群组 Enqueue 后消息到达；两个 client 中一个停止读取不会阻塞另一 client 的入队与接收；日志默认无 payload。

- [ ] **Step 4: README 文档**

记录新配置、`EnqueueToGroup` ACK 边界、Close、默认队列与 read limit；明确 `Accepted` 不代表客户端已处理。

- [ ] **Step 5: 公共库全量验证**

Run: `gofmt -w *.go`

Run: `go test ./... -count=1`

Run: `go test -race ./... -count=1`

Run: `go vet ./...`

Run: `git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --check`

Expected: 全 PASS；不修改 go.mod/go.sum；无暂存提交。

---

### Task 6: AirPlayTV API 重新接入与 Task 2 复审

**Files:**
- Modify: `D:/repo/github.com/airplayTV/api/controller/websocket_gateway.go`
- Modify: `D:/repo/github.com/airplayTV/api/controller/websocket.go`
- Modify: `D:/repo/github.com/airplayTV/api/controller/websocket_test.go`
- Modify: `D:/repo/github.com/airplayTV/api/controller/init.go` or bootstrap configuration only if required for safe manager config.
- Development-only: workspace linkage outside committed production files unless user approves a permanent replace.

- [ ] **Step 1: 更新 Gateway 契约 RED 测试**

Gateway 暴露一次快照入队结果；Controller ACK 使用 `Accepted`，不再先 List 再实时广播。测试慢客户端不延迟 ACK、Accepted=0 拒绝、Accepted>0 成功。

- [ ] **Step 2: 使用公共库 FIFO 和安全配置**

AirPlayTV `AppSocket` 使用 `NewWebsocketManagerWithConfig`，Debug 可用但 `LogPayload=false`。Presence/Close 顺序由公共库保证。

- [ ] **Step 3: 修复协议审查剩余项**

非法/空 request_id 不发送不可关联 ACK；新版带 request_id 控制必须有非空 string event；旧无 request_id 路径继续兼容。WebSocket runtime 必须进程唯一，sweeper 使用可取消根 context。

- [ ] **Step 4: 本地公共库联调**

使用临时 `go.work` 或命令级 workspace 指向 `D:/repo/github.com/lixiang4u/goWebsocket`；不静默写入生产 replace。运行 API 全量/race 和真实 WebSocket Presence→Close、A→B、慢 TV ACK 测试。

- [ ] **Step 5: 独立复审 Task 2**

重新生成 Task 2 report/diff，覆盖此前全部 Critical/Important。只有审查 APPROVED 才进入 H5 Task 3。
