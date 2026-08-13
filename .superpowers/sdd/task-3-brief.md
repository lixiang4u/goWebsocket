# Task 3 Brief: 每连接单 writer 与有界 outbound queue

## Goal

每个连接只允许一个 goroutine 调用 Gorilla WebSocket 写 API；发送与 Ping 通过实例级有界队列非阻塞入队。一个慢/坏连接不得阻塞其他连接或 manager 全局循环。

## Scope

- Create: `writer.go`, `writer_test.go`
- Modify: `connection.go`, `websocket.go`, `config.go`（仅必要初始化）
- 不实现公开 `EnqueueToGroup`（Task 4），但提供其所需的内部 `enqueueClient`/实例入队能力。
- 不实现 Manager Close（Task 5）。
- 不改变公开 API/导出 struct，不修改 go.mod/go.sum/index，不提交。

## Required behavior

1. 每连接创建一个 writer worker，容量为 `Config.OutboundQueueSize`；保存于 manager 私有 connection record，不进入 `ConnectionCtx`。
2. writer 是该 Gorilla socket 唯一调用 `SetWriteDeadline`、`WriteMessage` 的 goroutine；文本、Ping 都排同一 FIFO。
3. queue submit 必须非阻塞；满时返回 false，并隔离/关闭且仅注销该连接实例。不能阻塞 manager 或其他连接。
4. 写失败仅清理本实例；replacement 不受旧 writer 失败/迟到清理影响。
5. read EOF、event overflow、replacement、公开/内部 Disconnect 都应幂等停止 writer/socket，不 panic、不 double close channel。
6. writer 不能等待自身导致死锁；清理必须有明确 ownership。Task 5 前可没有 manager-wide Close，但每连接 worker 必须可退出且测试无泄漏。
7. Connect 只有在 writer 已创建并绑定实例后才发布 clients/record 和 Connect event；若初始化失败不得发布半连接。
8. `writeMessage` 旧 ticker 逻辑迁移到 writer：Ping 通过同一队列/worker 写出。保留既有 ticker 注释的语义与用户修改，不保留第二个 socket writer。
9. 数据帧在 enqueue 前编码一次并复制/不可变；写失败/队列满固定类别日志，不含 clientID、payload、URL、group、error 原文（除 LogForce 既有例外）。
10. Task 3 内部 API 应支持按固定 worker identity 入队，不能只按 client ID 后置查找导致 replacement 误投。

## TDD tests

- 注入 socket writer interface，精确验证文本/Ping FIFO、单 goroutine 写。
- A writer 被 barrier 阻塞时，B 能立即 enqueue 并写出。
- queue full 快速 false；随后只关闭 A，B 正常。
- 写失败只注销失败实例，replacement 不受影响。
- read EOF/Disconnect/replacement/重复 stop 不 panic，worker done 最终关闭。
- manager 全局 send 路径暂仍可调用内部 enqueue，但不得同步网络写；用慢 writer 证明 manager 能继续处理另一个命令。
- 真实 Gorilla 集成至少验证应用消息与 Ping 不产生 concurrent write，并能正常断开。
- 不使用 sleep 判断顺序；用 barrier/deadline helper。跑高重复和 race。

## Verification

```powershell
gofmt -w writer.go writer_test.go connection.go websocket.go config.go
go test ./... -run 'ConnectionWriter|OutboundQueue' -count=50
go test -race ./... -run 'ConnectionWriter|OutboundQueue' -count=20
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --check
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --cached --check
```

先记录 RED，完成后写 `task-3-report.md` 与 `task-3-diff.md`。
