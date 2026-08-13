# Task 4 报告：EnqueueToGroup 与旧发送 API 兼容

状态：`DONE`

## 实现

- 新增公开 `EnqueueResult` 与 `EnqueueToGroup(group, data)`；`Accepted` 仅表示帧已进入对应连接的 outbound queue。
- payload 只执行一次 `json.Marshal`；合法 `nil` 得到 `null` 且正常入队，编码错误通过独立 `bool` 返回值区分。
- 编码错误只记录固定分类 `[WebsocketMarshalError]`，不记录 payload、group 或 error 原文。
- group、uid、broadcast recipient ID 均排序；快照保存 client ID 与 event-worker identity，入队时再次校验 private record、public connection、socket 和 worker identity。
- 每个快照成员独立尝试非阻塞入队；queue full/closed/replacement 归入 `Rejected`，不会中止后续成员。
- `Accepted`/`Rejected` 每次新分配；空或不存在 group 返回两个非 nil 空 slice。
- 旧 `Send`、`SendToGroup`、`SendToUid`、`SendToAll` 保留 manager 串行提交边界，并增加完成 ACK：manager 完成 marshal/snapshot/enqueue 后公开方法才 dispatch 原有 user callback。callback 恰好提交一次，EventCtx 字段保持兼容。
- manager loop 的私有旧 channel 路径仅调用 marshal/snapshot/enqueue helper，不 dispatch callback，避免 send-to-group 入站业务 callback 被重复触发。

## TDD 证据

### RED

`go test ./... -run 'Enqueue|SendToGroup|LegacySend' -count=1`

- 退出码 `1`。
- 编译失败，缺少 `EnqueueToGroup`、`snapshotGroupRecipients`、`enqueueRecipients`，符合预期 RED。

### GREEN / 高重复 / race

- `go test ./... -run 'Enqueue|SendToGroup|LegacySend' -count=100`：PASS，`ok ... 1.320s`。
- `go test -race ./... -run 'Enqueue|SendToGroup|LegacySend' -count=50`：PASS，`ok ... 2.636s`。
- 补充 `ManagerConfig|ConnectionPublication` 高重复回归 `count=20`：PASS，`ok ... 2.319s`。

覆盖：空 group、marshal failure、安全日志、稳定排序、结果无别名、单次 marshal、相同 bytes、accepted/rejected、queue-full 后继续、close/replacement identity、旧四个 API callback/EventCtx、入站 send-to-group callback 不重复。

## 全量验证

- `go test ./... -count=1`：PASS，`ok ... 1.357s`。
- `go test -race ./... -count=1`：PASS，`ok ... 2.644s`。
- `go vet ./...`：PASS，无输出。
- 工作区与暂存区 `diff --check`：退出码 `0`；仅既有 Git 行尾提示。

## 独立复审修复：legacy callback 兼容语义

复审发现旧四个公开 Send API 忽略全局 worker 的非阻塞提交失败，`EventQueueSize` 很小时会出现 outbound 已入队但 callback 静默丢失。

### RED

- 新增 `EventQueueSize=1` 的确定性测试：阻塞首个 callback、占满唯一队列槽，同时并发执行后两个 Send API。
- 测试先确认四个 outbound frame 均已入队，再释放 handler；旧实现因后两个 callback 被丢弃而在等待第四个 callback 时失败。

第一版有界可靠提交虽然不丢 callback，但复审证明：global worker 正在执行 A callback、队列已有 B 时，A 重入 `SendToAll` 会等待自身队列槽位，从而永久自锁。这不兼容基线 `go v(ctx)` 的异步、可重入行为。

### 最终修复

- 新增私有 `dispatchLegacyUserEvent`：在 `handlerMu` 下捕获 handler；nil 直接返回；每次启动一个 goroutine 异步执行一次。
- goroutine 内部 recover handler panic，只记录固定 `[WebsocketEventHandlerPanic]`，不记录 payload、ID、error 或 recovered value。
- 公开 Send API 仍先等待 manager 完成 outbound marshal/snapshot/enqueue，再异步启动 callback 并立即返回；manager loop 与 writer 均不等待 callback。
- 该路径只用于无 `From` 的四个 legacy Send callback。有 `From` 的入站业务事件继续使用 per-connection FIFO 有界 worker，Connect/Close 继续使用 connection worker。
- 源码分类检查：正常 Bind/Unbind/Join/Leave 的 `dispatchUserEvent` 都携带 client ID；删除其创建 global worker 的无 `From` fallback，存在 handler 但缺少 `From` 时明确返回失败。缺失 handler 仍为 no-op。测试锁定不创建 global worker，避免未来误用再次引入静默丢 callback。
- 确定性回归覆盖 `EventQueueSize=1`、A callback 重入 `SendToAll`、并发 B/UID、全部调用完成、四种 callback 各一次、EventCtx 完整及 panic 安全日志。
- 兼容权衡：恢复旧版每 callback 一个 goroutine 的语义，因此极慢 callback 可累积 goroutine；这是既有兼容风险，而不是新增有界背压。Task 5 的 manager `Close` 不等待这些用户 callback，除非后续另行明确生命周期契约。

### 最终验证

- 最终聚焦（含普通事件无 `From` 拒绝回归）`count=100`：PASS，`ok ... 1.392s`。
- 最终聚焦 race `count=50`：PASS，`ok ... 3.046s`。
- `go test ./... -count=1`：PASS，`ok ... 1.380s`。
- `go test -race ./... -count=1`：PASS，`ok ... 2.794s`。
- `go vet ./...`、工作区及暂存区 `diff --check`：PASS。

## 边界

- manager-wide `Close` 和关闭后的 send 行为属于 Task 5。
- legacy callback goroutine 不纳入 Task 5 manager worker 等待范围；长期阻塞 handler 的资源风险与旧版一致。
- 未修改 `go.mod`/`go.sum`，未执行 `git add`、`commit`、`push`。
- 保留任务前已有的 staged `D go.mod`、未跟踪替代 `go.mod/go.sum`、ticker 注释和所有无关脏文件。
