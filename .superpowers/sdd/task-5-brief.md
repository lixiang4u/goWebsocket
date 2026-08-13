# Task 5 Brief: Manager Close、真实 WebSocket 集成与文档

## Goal

为 manager 增加幂等、并发安全的生命周期关闭，停止 register loop、连接 event worker、writer/heartbeat 和 socket；补真实 transport 集成与公共文档。

## Public API

```go
func (x *WebsocketManager) Close() error
```

## Required behavior

- manager 有私有 context/cancel、closed 状态、closeOnce、done；`Close` 幂等并等待 manager-owned goroutine 退出。
- Close 原子拒绝新 Connect/Bind/Join/Send/Enqueue 等操作；现有 void API Close 后快速 no-op，不 panic、不永久阻塞。`EnqueueToGroup` 返回空 Accepted/Rejected 或明确全 rejected（按文档固定一种）。
- register loop 所有 channel submit 必须可 select manager done；不能在 Close 与无缓冲 send 竞态中永久阻塞。
- Close 对每个当前实例按 identity：先停止接受、撤 public clients、停止 writer/heartbeat、关闭 socket、提交 terminal Close，等待 event workers 和 writers。Close handler 可以调用 manager API/Close 而不 self-deadlock；需明确设计和测试。
- 连接 event handler 若无限阻塞，Close 不可能同时有界等待。采用 context不可注入旧 handler；设计可选择等待并文档化，测试仅验证可释放 handler，或不等待用户 handler但安全终止worker。不得伪称可强制中止 Go handler。
- legacy Send* callback goroutine保持基线：Close 不等待它们；README明确。
- dispatcher/global 已无 global worker；worker map可快照并可靠 terminal。terminal已接受时不重复。
- Close 期间 writer failure/read EOF/公开 Disconnect/replacement 并发不重复 close、不误删、不死锁。
- 日志固定安全分类。
- 不改变旧 public API/struct；新增 Close/文档除外。不改 go.mod/go.sum/index，不提交。

## Integration tests

- Close before connections、重复/并发 Close。
- Close 停 register loop、writer heartbeat、event workers、socket；等待 release barrier 后返回。
- Close handler 重入 Send/Disconnect/Close 不死锁（若 `Close` 自身重入需定义：不能等待自己）。
- Close 与 read EOF、writer failure、queue full、replacement 并发。
- Close 后每个 public API 快速返回/no-op；Enqueue result语义固定。
- 真实 httptest：Connect -> A -> B -> Close FIFO；默认 64 KiB read limit（计划中“4 KiB”为笔误，以 config默认为准）；群组 enqueue 到达；慢/不读 client不阻塞另一个 client的 enqueue/receive；默认日志无 payload。
- 全部 barrier/deadline，无 sleep 判断；高重复/race。

## README

记录 ManagerConfig、新构造器、默认值、LogPayload安全、EnqueueToGroup accepted边界、Close语义、legacy callback goroutine边界。

## Verification

```powershell
gofmt -w *.go
go test ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle' -count=50
go test -race ./... -run 'ManagerClose|WebSocketIntegration|Lifecycle' -count=20
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --check
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --cached --check
```

写 `task-5-report.md`、`task-5-diff.md`。不得修改依赖/索引或提交。
