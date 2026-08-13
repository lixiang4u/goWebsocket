# Task 5 变更摘要

## `websocket.go`

- Manager 新增私有 context/cancel、atomic closed、closeOnce、done/registerDone、shutdown request、lifecycle lock、operation/read WaitGroup。
- 新增幂等 `Close`、public operation 登记和 register-loop 内 identity shutdown 快照。
- Handler 与 Close 通过 operation barrier 协调 `readWG.Add/Wait`。
- upgrade failure 日志固定分类。
- register loop 使用内部 `enqueueToGroup`，避免 public 生命周期线性化与 manager loop 自锁。

## `handler.go`

- 所有 public channel submit 使用 begin/end operation，并在锁外 select manager context。
- Close 后 void API no-op，legacy callback不再触发。
- public `EnqueueToGroup` 提供生命周期边界；内部 helper供 register loop复用。
- Legacy request 在 channel handoff 前可被 manager cancel；handoff 后必须等待 register loop ACK，保证实际 enqueue 与 callback 一致。

## `config.go`

- 新构造器创建 manager context、done/registerDone 和 shutdown channel。

## `event_dispatcher.go`

- 新增 worker 快照 helper，供 manager shutdown 对遗留 global worker 提交 terminal。
- 保持 Task 2 不变量：terminal handler 执行完成后才按 worker identity 从 dispatcher map 清理并关闭 worker done。
- worker 实例级 `overflowOnce` 在唯一真实 queue-full 分支记录固定安全分类，read/manager 两入口共享一次性边界。

## `lifecycle_test.go`

- 覆盖 concurrent/idempotent Close、public API after Close、register loop done、用户 handler不可强杀边界、Close handler重入、context snapshot/once、确定性无缓冲 channel取消与固定日志。
- 覆盖 request 已 handoff 且实际 enqueue 后，Close 等待 ACK、legacy callback 恰好一次。

## `event_dispatcher_test.go`

- 覆盖真实 WebSocket read path 与 manager Bind/Join path 的 queue-full 固定日志、同 worker once 和敏感字段缺失。

## `integration_test.go`

- 真实 WebSocket 覆盖分组入队、客户端收帧和 manager Close transport。

## `README.md`

- 文档化 ManagerConfig/default、LogPayload 安全、FIFO/single writer、Enqueue Accepted 边界、Close 与用户 handler/legacy callback 边界。
