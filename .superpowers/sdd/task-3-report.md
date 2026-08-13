# Task 3 报告：每连接单 writer 与有界 outbound queue

状态：`DONE`

## 实现

- 新增 `writer.go`：每连接一个 `connectionWriter`，唯一调用 transport 的 `SetWriteDeadline`/`WriteMessage`。
- 文本帧与 Ping 均非阻塞提交到同一个有界 FIFO；payload 在提交时复制。
- queue full 或 transport 写失败只触发一次固定分类回调，立即停止该实例；日志不含 client ID、payload、URL、group 或 error 原文。
- `connectionRecord` 私有保存 writer identity；`enqueueClient` 可显式校验 event worker identity，避免同 ID replacement 误投。
- register 在 writer 创建后才发布公开连接和私有 record；read EOF、event overflow、replacement、公开/内部 Disconnect 都通过幂等 `stop` 收敛。
- `stop` 不 drain 排队帧：当前已进入 transport 的写允许在 deadline 内结束，排队帧丢弃。writer 不等待自身；失败回调异步提交 manager 注销。
- heartbeat 仅作为队列 producer，退出后关闭 `heartbeatDone`；writer `done` 表示 writer 与 heartbeat 均已退出。
- manager 的 client/group/uid/broadcast 旧发送路径只做编码与非阻塞入队，不再同步执行网络写。

## 独立复审修复

- `Important`：queue-full 观察与 stopped 转换已合并到同一个 `writer.mu` 临界区；该临界区内设置 `stopped` 并关闭 `stopCh`，锁外通过 `failOnce` 执行回调。确定性 hook 测试在 full 已观察但回调未执行时释放队列容量，验证并发及后续 enqueue 全部返回 false。
- `Important`：连接发布改为先建立完整私有 record（socket/worker/writer/ready），再发布 clients。`enqueueClient` 同时校验 record ready、worker identity、clients 已公开且 socket identity 一致；因此 record-first 的 first/replacement 窗口都不会向尚未公开的新 writer 投递。公共 Send/SendToAll 通过 manager 无缓冲 channel 与 register 串行，只有 clients 发布完成后才会被处理。
- 注销顺序改为先移除公开 clients，再按 worker identity 移除 private record；注销窗口内 enqueue 因公开索引缺失而拒绝。

## TDD 证据

### RED

命令：`go test ./... -run 'ConnectionWriter|OutboundQueue' -count=1`

结果：退出码 `1`，因缺少 `outboundFrame`、`newConnectionWriter`、`writerFailure` 等生产符号而编译失败，属于预期 RED。

### GREEN 与并发验证

- `go test ./... -run 'ConnectionWriter|OutboundQueue|ConnectionPublication' -count=100`：PASS，`ok ... 1.995s`。
- `go test -race ./... -run 'ConnectionWriter|OutboundQueue|ConnectionPublication' -count=50`：PASS，`ok ... 3.037s`。
- `go test ./... -count=1`：PASS，`ok ... 1.507s`。
- `go test -race ./... -count=1`：PASS，`ok ... 2.561s`。
- `go vet ./...`：PASS，无输出。

测试覆盖：文本/Ping FIFO、唯一 writer、payload copy、慢 A/正常 B 隔离、queue full 原子停止及并发拒绝、写失败一次性回调、重复 stop、stop 丢弃排队帧、旧失败不移除 replacement、原子连接发布窗口、manager loop 不等待网络写、真实 Gorilla 文本/Ping、read EOF 停止 writer 并移除实例。

## 边界与剩余工作

- 公开 `EnqueueToGroup`、稳定排序和 accepted/rejected 结果属于 Task 4，未在本任务实现。
- manager-wide `Close` 与统一等待全部连接 worker 属于 Task 5；当前每连接 `done` 已可确定性等待。
- 未修改 `go.mod`/`go.sum`，未执行 `git add`、`commit` 或 `push`；保留所有任务前脏文件与 ticker 注释语义。
