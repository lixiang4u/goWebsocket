# Task 2 报告：同连接 FIFO 事件执行器

状态：`DONE`

## 实现文件

- 新增 `event_dispatcher.go`：连接实例级有界 FIFO worker、独立 terminal 保证路径、panic 隔离、实例 identity 清理。
- 新增 `event_dispatcher_test.go`：FIFO、跨连接并行、panic 恢复、队列满、terminal、同 ID replacement、handler 捕获及真实 Handler 顺序测试。
- 修改 `websocket.go`：事件 handler RWMutex；入站事件同步提交；注册/注销 ACK；Handler 在 Connect 已入队后启动 read loop；队列溢出关闭对应实例。
- 修改 `config.go`：按归一化 `EventQueueSize` 初始化 dispatcher，并初始化同步注册/注销通道。

## 设计不变量

- worker identity 是连接实例指针，不是可复用 client ID；map 清理必须同时匹配 ID 和实例。
- worker 保存自身 socket 和初始 `FromCtx`；旧实例溢出/断开不能查询并关闭同 ID replacement socket，也不能把新实例上下文交给旧 Close。
- Handler 通过 `registerSync` 进入既有 manager 串行循环；只有 `connect` 完成且 Connect task 已提交后 ACK，ACK 后才启动 write/read goroutine。
- read loop 按帧读取顺序同步提交；退出通过 `unregisterSync` 将 Close terminal 排到此前已接受任务之后。
- 普通任务使用有界 channel 非阻塞提交；terminal 不占普通队列槽，而是在关闭普通队列后由 worker 唯一执行，因此队列已满时仍不会丢失。
- terminal 一旦接受，后续提交全部拒绝；terminal handler 返回或 panic 后才清理 worker。
- handler 在提交时于 RWMutex 下捕获；后注册覆盖仅影响之后提交的事件。
- handler panic 被逐任务 recover，日志固定为 `[WebsocketEventHandlerPanic]`，后续任务继续执行。

## TDD 证据

### RED

命令：`go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

结果：退出码 `1`；缺少 `newEventDispatcher`、`userEventTask` 等待实现符号，属于预期 RED。

### GREEN

最终命令：`go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 1.374s`。

### Race

最终命令：`go test -race ./... -run 'EventDispatcher|ClientEventFIFO' -count=1`

结果：退出码 `0`：`ok github.com/lixiang4u/goWebsocket 2.504s`。

### 全量回归

- `go test ./... -count=1`：退出码 `0`，`ok ... 1.385s`。
- `go test -race ./... -count=1`：退出码 `0`，`ok ... 2.575s`。
- `git ... diff --check`：退出码 `0`。
- `git ... diff --cached --check`：退出码 `0`。

第一次在受限沙箱复跑时，四个 Go 命令均因系统 `go-build` 缓存 `Access is denied` 失败；未发生源码编译/测试失败。随后仅授权访问既有 Go 构建缓存重跑，上述四项全部通过。

## 剩余风险

- Manager 尚无 `Close` 生命周期；dispatcher/register goroutine 的统一退出由计划 Task 5 实现。
- writer 仍为旧实现，网络写隔离及唯一 writer 由计划 Task 3 实现。
- 公共 `Connect` 保持旧异步 API 语义；真实 HTTP Handler 使用内部 ACK 屏障保证 read loop 顺序。
- 全局无连接来源回调使用独立 global worker；其队列满时保持非阻塞失败，但旧 void API 无法向调用者报告，后续 API 重构需继续保持隔离。

## 独立复审修复

独立复审提出的 3 个 Important 和 1 个 Minor 已全部进入 RED/GREEN 闭环：

- 第一轮 RED：新增 replacement、Close 完整上下文、nil global handler 测试后，编译因缺少连接实例 identity 失败。
- 原子 replacement：manager 私有 `connectionRecords` 同时保存 socket 和 worker identity；导出 `ConnectionCtx` 完全保持原来的三个字段及顺序。manager 串行注册新实例时，先按旧私有 identity 移除旧 uid/group 索引、关闭旧 socket、提交旧 terminal，再发布新公开连接快照和私有 identity。旧 worker 最终清理仍做指针比较，不会删除新 worker。
- `Disconnect`：恢复旧异步返回边界，不在调用方读取 clients，也不等待处理 ACK。调用只向原 manager 无缓冲串行通道发送 ctx；manager 接收后按 registerEvent 总序解析私有 current identity。若 `ctx.Socket` 非空则严格匹配，不匹配为迟到旧断开并 no-op；nil socket 保持旧语义，即断开 manager 处理该命令时的当前实例。内部 read/overflow 仍携带固定 worker identity 并同步等待注销完成。
- Close 快照：worker 对初始 `FromCtx` 深拷贝，保存 UserAgent、RequestURI、Addr；bind/unbind/join/leave 后仅更新该实例的 Uid/Groups。Close 总是从 worker 快照 clone，不读取同 ID replacement 的动态字段。
- global worker：先在 handler RWMutex 下查找 handler；nil handler 直接成功 no-op，不再创建 global worker/goroutine。
- 真实 transport：新增 `httptest.Server` + Gorilla WebSocket 测试，使用事件 channel 断言 `Connect -> A -> B -> Close`、`Connect -> Presence -> Close` 和 Close once；另用真实 WebSocket pair 验证 replacement、迟到旧 Disconnect、新 socket 持续收发、旧/新 worker 最终清理。

复审修复后的最终验证：

- `go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=20`：`ok ... 1.657s`。
- `go test -race ./... -run 'EventDispatcher|ClientEventFIFO' -count=5`：`ok ... 2.672s`。
- `go test ./... -count=1`：`ok ... 1.487s`。
- `go test -race ./... -count=1`：`ok ... 2.576s`。
- `go vet ./...`：退出码 `0`，无输出。
- 工作区和暂存区 `diff --check`：退出码 `0`；仅有既有 Git 换行提示。

## 第二轮源码兼容与时序复审

- RED：恢复 `ConnectionCtx` 原字段后，旧实现对 `eventWorker` 的引用导致编译失败；随后将 identity 全部迁入 manager 私有索引。新增外部包 `compatibility_external_test.go`，使用 `goWebsocket.ConnectionCtx{nil, map[string]bool{}, ""}` 验证非命名复合字面量源码兼容。
- 增加 manager 串行循环测试 hook，确定性冻结在“Connect 通道已接收、连接尚未发布”和“Disconnect 通道已接收、注销尚未执行”窗口。测试证明 `Connect(); Disconnect()` 最终按总序注销，并且两项公开方法都在 manager 接收命令后返回，不被处理 ACK 延长。
- 所有测试侧等待改用 3 秒 deadline helper；保留 handler 内部 barrier。Close-once 在对应 worker 从 dispatcher 完成清理后读取原子计数，不再使用立即 `default` 伪断言。
- 第二轮最终 `go test ./... -run 'EventDispatcher|ClientEventFIFO' -count=50`：`ok ... 1.662s`。
- 第二轮最终 `go test -race ./... -run 'EventDispatcher|ClientEventFIFO' -count=20`：`ok ... 2.725s`。
- 第二轮全量：`go test ./... -count=1` 为 `ok ... 1.406s`；`go test -race ./... -count=1` 为 `ok ... 2.577s`；`go vet ./...`、工作区和暂存区 `diff --check` 均退出码 `0`。

## Git 边界

- 未修改 `go.mod`/`go.sum`，未执行 `git add`、`commit` 或 `push`。
- 保留任务前已有 `D  go.mod` 与未跟踪替代 `go.mod/go.sum` 的异常状态。
- 保留 `_examples`、`.idea`、二进制/APK/JSON 等全部无关脏文件。
- 保留 `websocket.go` 既有 ticker 区域及其注释。
