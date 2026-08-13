# Task 5 报告：Manager Close、真实 WebSocket 集成与文档

状态：`DONE`

## 实现

- 新增 `(*WebsocketManager).Close() error`：并发安全、幂等；原子设置 closed、取消 manager context，并等待 register loop、已登记 public operation、connection writer/heartbeat 和 read loop 退出。
- public mutation 使用 `beginOperation/operationWG` 线性化：生命周期锁只覆盖 closed 检查和 inflight 登记；所有无缓冲 channel submit 在锁外 `select context.Done()`，Close 不会与已退出 register loop 形成锁环。
- `Handler` 在 upgrade 后登记 public operation；成功注册后、结束 operation 前执行 `readWG.Add`。Close 先 `operationWG.Wait` 再 `readWG.Wait`，禁止 `Add` 与 `Wait` 竞态。
- Close 按当前 `connectionRecord` identity 注销：停止 writer/heartbeat、撤销公开索引、关闭 socket、提交唯一 terminal Close；read EOF、writer failure、replacement 和迟到 Disconnect 沿用 identity 校验，不能误删 replacement。
- 关闭后的 Connect/Disconnect/Bind/Unbind/Join/Leave/Send API 快速 no-op；`On` 返回 false；`EnqueueToGroup` 返回非 nil 空 Accepted/Rejected。
- manager Close 不等待用户 handler：Go 无法强制中断 handler。普通 FIFO 和 terminal 仍按原有 Task 2 语义执行，Close handler 返回后 worker 才从 dispatcher map 按 identity 清理。因 manager core 的 done 不依赖 event worker，Close handler 可安全重入 Send/Disconnect/Close。
- legacy Send callback goroutine不纳入 manager lifecycle 等待，保持 Task 4/旧版兼容边界。
- WebSocket upgrade 错误日志改为固定 `[WebsocketUpgradeError]`，不记录 Gorilla 动态错误、请求路径或请求数据。
- 重写 README：配置/default、安全日志、FIFO/single writer、Enqueue accepted 边界、Close/handler/callback 生命周期均已记录。

## TDD 证据

### Close RED

命令：`go test ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle' -count=1`

结果：退出码 `1`；编译器报告 `manager.Close` 与 `registerDone` 未定义，属于预期 RED。

### 安全日志 RED

命令：`go test ./... -run 'LifecycleUpgradeFailureLog' -count=1`

结果：退出码 `1`；测试捕获到 `[WebsocketUpgradeError] websocket: the client is not using...` 动态详情。改为固定分类后进入 GREEN。

### GREEN / 高重复 / race

- `go test ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle|LegacySend|Enqueue' -count=50`：PASS，`ok ... 1.362s`。
- `go test -race ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle|LegacySend|Enqueue' -count=20`：PASS，`ok ... 2.761s`。
- 确定性生命周期测试冻结 register loop，使第二个 public Bind 阻塞在无缓冲 channel submit；Close 必须先取消该 submit，释放 barrier 后再结束。该测试不依赖 sleep 推断锁序。
- 并发 legacy Send 回归曾确定性暴露 public Enqueue 生命周期锁与 register loop 的锁环；拆分 public `EnqueueToGroup` 与 manager 内部 `enqueueToGroup` 后，高重复/race 通过。

## 真实 transport 与生命周期覆盖

- 新增 `integration_test.go`：真实 `httptest.Server` + Gorilla client，验证连接、分组、`EnqueueToGroup` Accepted、网络收到正确帧，以及 manager Close 关闭 transport。
- 新增 `lifecycle_test.go`：Close-before-connect、16 并发 Close、register loop done、Close 后所有 public mutation、阻塞用户 handler 边界、Close handler 重入 Close/Send/Disconnect、Close context 快照和 exactly once、public channel cancel、固定升级错误日志。
- Task 1 既有真实 transport 测试覆盖每 manager 64 KiB 默认/自定义 read limit与安全 payload 日志。
- Task 2 既有真实 transport 测试覆盖 Connect -> A -> B -> Close FIFO、Presence 紧邻 Close、replacement/迟到 Disconnect。
- Task 3 既有真实 transport/注入 transport 测试覆盖单 writer、Ping/文本 FIFO、慢连接隔离、queue full、write failure、read EOF。

## 最终验证

- 显式根目录 Go 文件列表 `gofmt -w`：完成。
- `go test ./... -count=1`：PASS，`ok ... 1.346s`。
- `go test -race ./... -count=1`：PASS，`ok ... 2.748s`。
- `go vet ./...`：PASS，无输出。
- 工作区 `diff --check`：PASS，仅既有 Git LF/CRLF 警告。
- 暂存区 `diff --cached --check`：PASS，仅既有 Git LF/CRLF 警告。

## Git 与兼容边界

- 未修改 `go.mod`/`go.sum`，未执行 `git add`、`commit`、`push`；保留任务前已有 staged `D go.mod` 与未跟踪替代 `go.mod/go.sum`。
- 保留 `_examples`、`.idea`、APK/JSON/二进制等无关脏文件。
- `websocket_test.go` 因最终显式 gofmt 被 Git stat 标为 `M`，但 `git diff -- websocket_test.go` 为空，工作树 `hash-object` 与索引 blob 均为 `f6722f68c18b21ad0c5679fdc5f897b1501d44ab`；没有内容变化，也未刷新/修改索引。
- 旧 public 方法签名和 `ConnectionCtx` 三字段/字段顺序未改变；仅新增 `Close` 公开方法。

## 独立复审三项 Important 修复

### 1. Legacy request handoff 与 ACK

- 根因：旧 `submitSend` 在无缓冲 channel handoff 成功后仍同时等待 request ACK 和 manager cancel。Close 取消 context 时可能随机返回 false；register loop 随后已经完成真实 outbound enqueue，但 legacy callback 被静默跳过。
- 修复：重命名为 `submitLegacyRequest`。handoff 前继续 `select manager done/channel`；handoff 成功后无条件等待该 request 的 `done` ACK。该 operation 已登记在 `operationWG`，Close 必须等 ACK 完成。
- 确定性回归：`beforeSendRequestAck` 在 register loop 已完成 enqueue、尚未 ACK 的唯一窗口冻结；启动 Close 并确认 context 已取消，释放 ACK 后验证 outbound 恰好一帧、legacy callback 恰好一次、Send 与 Close 都完成。

### 2. Close 等待契约统一

- `Close` 只等待 manager core、已登记 public operation、read loop、writer 和 heartbeat。
- `Close` 明确不等待 connection FIFO、terminal handler 或 legacy callback。Go handler 无法强杀；已接受 FIFO/terminal 异步完成，terminal handler 返回后 worker 才按 identity 清理。
- Godoc、README 和本报告已统一，不再声称 Close 等待 event scheduler/worker。

### 3. Event queue full 一次性安全日志

- `clientEventWorker` 新增实例级 `overflowOnce`；固定 `[WebsocketEventQueueFull]` 在 `worker.submit` 唯一真实 queue-full default 分支记录。
- read loop 与 manager Bind/Join 等两个入口即使竞态观察同一满队列，同一 worker identity 也只记录一次；replacement worker 有独立 once。
- 日志不携带 client ID、uid、group、payload、error 或 recovered value。
- 测试覆盖真实 Gorilla read path 和 manager event path，验证 count=1 及敏感字段缺失。

复审修复聚焦验证：

- `go test ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle|EventQueueFull|LegacySend|Enqueue' -count=50`：PASS，`ok ... 1.778s`。
- `go test -race ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle|EventQueueFull|LegacySend|Enqueue' -count=20`：PASS，`ok ... 2.951s`。
- `go test ./... -run 'ManagerCloseWaitsForHandedOff|EventQueueFull' -count=100`：PASS，`ok ... 1.530s`。
- `go test -race ./... -run 'ManagerCloseWaitsForHandedOff|EventQueueFull' -count=50`：PASS，`ok ... 2.919s`。
