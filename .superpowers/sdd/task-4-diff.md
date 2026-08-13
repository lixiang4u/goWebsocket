# Task 4 变更摘要

## `protocol.go`

- 新增公开 `EnqueueResult{Accepted, Rejected []string}`。

## `handler.go`

- 新增 `EnqueueToGroup`、安全 marshal helper、稳定 recipient snapshot 与统一 enqueue helper。
- group/user/all/client 快照按 ID 排序并捕获具体 event-worker identity。
- 旧四个公开 Send API 通过同步 manager request 完成入队后，再提交一次原有 callback。

## `websocket.go`

- 新增四个内部同步 send request channel；保留既有四个私有 EventCtx channel 类型及非缓冲语义。
- manager loop 所有发送分支复用统一 marshal/snapshot/enqueue 路径，不执行同步 transport write，不重复 dispatch callback。
- legacy Send callback 使用独立 `dispatchLegacyUserEvent` 恢复基线 goroutine 异步调用；panic 仅输出固定安全分类日志。
- 私有测试 hook 在 callback recover/退出后发出确定性完成信号，不改变公开 API。
- 普通 `dispatchUserEvent` 对存在 handler 但缺少 `From` 的调用明确失败，不再创建 global worker；nil handler 保持 no-op。

## `event_dispatcher.go`

- 最终未加入 global blocking submit；有 `From` 的入站业务事件仍沿用 Task 2 的 per-connection FIFO 有界 worker。

## `connection.go`

- `_send` 改为复用安全 marshal 与单 recipient identity snapshot/enqueue helper。

## `config.go`

- 初始化 Task 4 的四个内部同步 send request channel；既有 channel 保持不变。

## `enqueue_test.go`

- 覆盖稳定排序、非 nil 空结果、无别名、单次 marshal、相同 bytes、混合 accepted/rejected、queue-full 继续、replacement/close identity、安全错误日志。
- 覆盖旧 Send API 的完成顺序、callback 恰好一次、EventCtx 兼容、旧 group 单次 marshal，以及入站 send-to-group callback 不重复。
- 使用 worker marker/barrier 和 deadline，不依赖固定 sleep 推断时序。
- 新增 `EventQueueSize=1` 重入测试：A callback 阻塞后重入 `SendToAll`，同时并发 group/uid Send；验证所有调用完成、四种 callback 各一次、EventCtx 兼容和 panic 隔离日志不泄露 recovered value。
