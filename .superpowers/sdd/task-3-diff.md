# Task 3 变更摘要

## `writer.go`（新增）

- 定义私有 `messageWriter` transport 接口，`*websocket.Conn` 可直接满足。
- 定义 `connectionWriter`、`outboundFrame`、固定 `writerFailure` 分类。
- 实现有界非阻塞 enqueue、payload copy、单 writer FIFO、heartbeat 入队、幂等 stop/fail 和完整 done 生命周期。
- queue full 在状态锁内原子切换 stopped/关闭 stopCh，失败回调在锁外 failOnce 执行。

## `writer_test.go`（新增）

- 注入 recorder/barrier transport 验证 FIFO、单写、跨连接隔离、queue full、失败/stop 幂等和 stop 丢弃语义。
- 验证旧实例失败不能影响 replacement、manager loop 不被慢写阻塞。
- 使用真实 Gorilla pair 验证文本/Ping 和 read EOF 生命周期。
- 通过确定性 hook/barrier 验证 queue-full 后并发 enqueue 必须失败，以及 private record/public clients 两阶段发布窗口不泄漏或丢失串行发送。

## `connection.go`

- `connect` 在发布连接前创建实例 writer，并保存于私有 connection record。
- 完整 private record 先发布，public clients 后发布；内部 enqueue 同时校验 ready、worker 和 public socket identity。
- disconnect 先撤销 public clients，再按 worker identity 撤销 record，所有中间窗口均 fail closed。
- `_send` 改为编码后调用内部非阻塞 `enqueueClient`，不再直接写 socket。

## `websocket.go`

- `connectionRecord` 增加私有 writer identity。
- group/uid/broadcast 每次命令只编码一次并逐实例非阻塞入队。
- unregister 幂等停止匹配实例的 writer；写失败按实例 identity 异步注销。
- 删除旧 `writeMessage` 第二 writer/ticker；Handler 只启动 read loop，心跳迁入 `connectionWriter`。
