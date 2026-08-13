# Task 1 Brief: 配置与安全日志

Repository: `D:\repo\github.com\lixiang4u\goWebsocket`

Files: create `config.go`, `config_test.go`; modify `websocket.go`.

## Public interfaces

```go
type ManagerConfig struct {
    Debug bool
    LogPayload bool
    ReadLimitBytes int64
    OutboundQueueSize int
    EventQueueSize int
}
func NewWebsocketManagerWithConfig(config ManagerConfig) *WebsocketManager
func NewWebsocketManager(debug ...bool) *WebsocketManager
```

Defaults: ReadLimitBytes=`64 << 10`, OutboundQueueSize=64, EventQueueSize=64. Non-positive values normalize to defaults. Old bool constructor maps only to Debug; LogPayload stays false. Preserve current channels/maps/background loop for now.

Logging: Debug=false logs nothing. Debug=true plus LogPayload=false must not contain raw JSON or field values; protocol errors use fixed category and optional byte count. Only explicit LogPayload=true may include raw inbound payload. LogForce unchanged.

TDD: tests first for defaults, normalization, old constructor, payload secrecy and opt-in. Capture global logger with `t.Cleanup`, no parallel tests. RED then GREEN: `go test ./... -run 'ManagerConfig|PayloadLogging' -count=1`; also race. Run gofmt and diff --check.

Constraints: preserve existing websocket.go commented ticker log. Do not modify go.mod/go.sum or their index state. Preserve unrelated dirty files. UTF-8 no BOM. No add/commit/push. Use apply_patch.

Deliverables: `.superpowers/sdd/task-1-report.md` and `.superpowers/sdd/task-1-diff.md`. Report exact RED/GREEN/race, files, compatibility/security audit and concerns.
