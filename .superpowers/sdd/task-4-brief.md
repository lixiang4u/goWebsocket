# Task 4 Brief: EnqueueToGroup 与旧发送 API 兼容

## Goal

提供一次群组快照的精确非阻塞入队结果，供调用方生成 delivery ACK；旧 `Send*` void API 复用同一安全入队机制并保持事件回调语义。

## Public API

```go
type EnqueueResult struct {
    Accepted []string
    Rejected []string
}

func (x *WebsocketManager) EnqueueToGroup(group string, data interface{}) EnqueueResult
```

`Accepted` 只表示帧成功进入对应连接 outbound queue，不表示网络写出或客户端处理。

## Scope / invariants

- Modify: `protocol.go`, `handler.go`, `websocket.go`
- Create: `enqueue_test.go`
- 不实现 manager-wide Close（Task5）。不改变既有公开签名/struct 字段；新增 API 除外。
- `ToBuff(data)` 只执行一次；结果作为不可变 bytes 供各 writer enqueue（writer 自身仍复制）。编码失败需可检测：若现有 `ToBuff` 吞错，新增私有 marshal helper，但不改变 `ToBuff` 兼容行为。
- 获取 group 成员 client ID 快照后排序；返回的新 slices 不暴露内部数据。
- 对快照中每个 ID，以当时 clients+private record identity 非阻塞 enqueue；成功进 Accepted，连接关闭/替换/queue full 进 Rejected。
- 空/不存在 group 返回两个非 nil 空 slice；编码失败全部为空并固定分类日志，不泄露 data/error。
- `Send`, `SendToGroup`, `SendToUid`, `SendToAll` 不再让 manager 循环做同步网络写；均通过相同 snapshot/enqueue helper。
- 旧 API 对应 user callback 仍恰好触发一次，事件名称与 EventCtx 字段保持；handler 提交失败仍按既有事件队列策略隔离。
- send-to-group 入站用户事件的旧业务 callback 不能因新 API 被重复触发。
- accepted/rejected 结果在返回后不因内部变化改变。

## Tests (TDD)

- 空/不存在 group；marshal failure；排序且不别名。
- 两个 recipients：一个 accepted，一个 queue full/closed rejected；慢 writer 不延迟返回。
- snapshot 后 replacement/close 的 identity 行为。
- payload 只 marshal 一次（自定义 Marshaler 计数），每个 writer 收到相同 bytes。
- 旧四个 Send API 使用 enqueue，无同步 transport write；事件 callback 各一次且 EventCtx 兼容。
- 入站 send-to-group callback 不重复。
- 高重复/race；barrier/deadline，不用 sleep。

## Verification

```powershell
gofmt -w protocol.go handler.go websocket.go enqueue_test.go
go test ./... -run 'Enqueue|SendToGroup|LegacySend' -count=100
go test -race ./... -run 'Enqueue|SendToGroup|LegacySend' -count=50
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --check
git -c safe.directory=D:/repo/github.com/lixiang4u/goWebsocket diff --cached --check
```

写 `task-4-report.md`、`task-4-diff.md`。不改 go.mod/go.sum/index，不提交。
