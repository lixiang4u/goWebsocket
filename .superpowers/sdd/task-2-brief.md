# Task 2 Brief: 同一连接 FIFO 事件执行器

## Goal

在不改变公共 API 的前提下，使同一 WebSocket 连接的 Connect、入站业务事件、Close 严格 FIFO；不同连接仍可并行。移除当前 `go handler(ctx)` 导致的 Presence/Close 与切房乱序。

## Scope

- Create: `event_dispatcher.go`
- Create: `event_dispatcher_test.go`
- Modify: `websocket.go`
- 如确保 Connect 先于 read loop 必须，可对 Handler 的启动顺序做最小调整。
- 不做 writer/outbound queue、EnqueueToGroup、Manager Close（后续任务）。
- 不修改 `go.mod`/`go.sum`，不 add/commit/push；保留既有 ticker 注释和所有无关脏文件。

## Required behavior

1. 每个连接实例有独立有界 FIFO worker，队列容量来自规范化后的 `ManagerConfig.EventQueueSize`。
2. 同一连接严格观察到 `Connect -> frame1 -> frame2 -> Close`；Connect 必须在启动该连接 read loop 前成功提交。
3. read loop 对每帧按读取顺序同步提交任务；read 退出后提交 terminal Close，Close 必须排在此前已接受任务之后且不能丢失。
4. terminal 一旦被接受，同一连接实例拒绝后续任务；terminal handler 完成后才清理 worker。
5. worker map 删除必须校验实例 identity，避免旧连接的 terminal 删除复用相同 client ID 的新 worker。优先使用连接实例 identity/key，不要仅以可复用字符串 ID 识别生命周期。
6. 不同连接 handler 可并行；同一连接 handler 不并行。
7. handler panic 必须 recover，记录固定类别日志（不含 ctx/payload/ID），同一连接后续任务继续执行。
8. 普通队列满时非阻塞返回 false，并关闭/隔离该异常连接；不得阻塞其他连接。terminal 必须有可证明的保证路径，不能因普通队列已满而静默丢 Close。
9. handler 注册/读取加适当 RWMutex；保留后注册 handler 覆盖旧 handler 的语义。任务应在提交时捕获确定的 handler，避免执行时被覆盖导致历史事件改派。
10. 无连接来源的内部回调若现有代码确有需要，使用独立 global worker，不得破坏真实连接隔离。

## Tests (TDD, barriers/channels; no sleep-based order assertions)

- 同一连接 Connect -> A -> B -> Close 精确顺序。
- Presence -> immediate Close 不会 Close 先执行；A-room -> B-room 不会倒序。
- 两个连接在 A handler 阻塞时，B 仍能执行。
- handler panic 后同连接下一事件继续。
- terminal 后拒绝新任务，且 terminal 完成后 worker 清理。
- 队列满快速返回，不阻塞其他连接；Close 仍得到一次且仅一次执行。
- 相同 client ID 的新连接不被旧 terminal worker 清理。
- 验证 Handler 启动顺序：Connect 已提交后才允许 read loop 产生业务事件。

## Verification

```powershell
gofmt -w event_dispatcher.go event_dispatcher_test.go websocket.go
go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=1
go test -race ./... -run 'EventDispatcher|ClientEventFIFO' -count=1
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --check
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --cached --check
```

先记录 RED，再实现 GREEN。报告列出文件、设计不变量、RED/GREEN/race 输出、剩余风险和 Git 边界。
