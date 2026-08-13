# Task 2 变更摘要

## `event_dispatcher.go`（新增）

- `userEventTask` 在提交时固化 handler、上下文和 terminal 属性。
- `clientEventWorker` 使用容量为 `ManagerConfig.EventQueueSize` 的普通任务队列。
- terminal 通过 worker 内受锁保护的独立槽位保证一次接受、一次执行，不受普通队列已满影响。
- worker map 删除校验具体 worker 指针；同 ID 新实例不会被旧 terminal 删除。
- worker 绑定其连接 socket 和初始 `FromCtx`，隔离旧实例的关闭与 Close 快照。
- 每任务 panic recover；不同 worker 各自 goroutine 并行，同一 worker 串行。

## `websocket.go`（修改）

- 新增 handler RWMutex，`On` 保持后注册覆盖语义，dispatch 在提交时捕获 handler。
- 新增 manager 串行循环内的同步注册/注销请求和 ACK。
- `Handler` 等待注册 ACK 后再启动 read/write；ACK 表示索引已连接且 Connect task 已入连接 worker。
- `readMessage` 按读帧顺序提交到固定 worker，读退出同步提交 Close terminal。
- 普通事件队列满时关闭该 worker 绑定的 socket，并经实例校验注销；不得关闭同 ID replacement。
- 连接级事件使用对应 worker；无 `From` 的内部回调使用独立 global worker。
- 保留 Task 1 的 read limit 和安全日志变更，未改 ticker 注释。

## `config.go`（修改）

- 构造器初始化 event dispatcher、固定 panic 分类日志回调，以及同步注册/注销通道。

## `event_dispatcher_test.go`（新增）

- 精确验证 Connect/A/B/Close FIFO 和 terminal 后拒绝。
- 用 channel barrier 验证跨连接并行，不依赖 sleep 推断顺序。
- 验证 handler panic 后下一事件继续。
- 验证普通队列满快速返回且 Close 仍恰好执行。
- 验证旧 terminal 不删除同 ID replacement worker。
- 用真实 WebSocket pair 验证旧溢出只关旧 socket，replacement 仍可收发、索引仍存在、旧 Close 使用旧上下文。
- 验证 handler 提交时捕获及后注册覆盖边界。
- 用 `httptest` + Gorilla client 验证 Connect handler 阻塞时业务 handler 不会提前执行。

## 独立复审后的补充变更

- `protocol.go`：`ConnectionCtx` 保持原始 `Socket/Group/Uid` 三字段及原顺序；未增加任何字段。
- `connection.go`：公开连接快照与 manager 私有 `connectionRecord` 由同一串行注册操作协调发布；注销仅在私有记录 worker 指针匹配时移除公开索引。
- `handler.go`：公开 `Disconnect` 不预读 clients、不等待 ACK，只向 manager 串行通道发送 ctx，恢复旧异步返回边界。
- `websocket.go`：unregister 消息携带 worker identity；replacement 先终止旧实例再发布新实例；Close 使用 worker 自身完整快照；bind/group 操作同步刷新对应 worker 动态字段；nil global handler 在创建 worker 前返回。
- `event_dispatcher.go`：worker 提供加锁的动态上下文更新和深拷贝快照，Groups 排序以保证稳定输出。
- `config_test.go`：适配携带 identity 的内部 unregister 通道类型，仍验证通道保持无缓冲，未改变公共配置语义。
- `event_dispatcher_test.go`：增加真实 WebSocket 完整 FIFO、Presence/Close、Close once、replacement、迟到 Disconnect、完整握手元数据和 nil global worker 回归。

## 第二轮复审补充

- `compatibility_external_test.go`：外部测试包使用三元素非命名 `ConnectionCtx` 复合字面量，锁定源码兼容。
- `websocket.go`：新增私有 `connectionRecords`；公开 Disconnect 由 manager 接收后按总序解析 identity，expected socket 非空时严格匹配；测试 hook 仅为确定性验证接收/发布窗口。
- `event_dispatcher_test.go`：新增 Connect 接收但未发布窗口和 Disconnect 接收但未处理窗口测试；证明串行顺序与旧异步返回边界。所有测试等待使用 deadline helper，Close once 等 worker 清理完成后检查原子计数。
