# Legacy heartbeat patch

Maintenance branch: `legacy-20250321-heartbeat-fix`.

Base: `01685938649f5056418eb8573f53f54ee52d8e92`
(`v0.0.0-20250321082802-01685938649f`).

This branch replaces the heartbeat's `SetWriteDeadline` and `WriteMessage`
with `WriteControl`, whose deadline is passed directly. Gorilla permits
`WriteControl` concurrently with application writes. The legacy event names,
dispatch behavior, limits, dependencies, and business-message path are unchanged.
Do not merge master into this branch; review future legacy fixes individually.

## Consumer installation

Use `go get github.com/lixiang4u/goWebsocket@<patch-commit>` in the consumer
module, then review and commit its `go.mod` and `go.sum`. Go resolves the commit
to an immutable pseudo-version. Do not use `@latest` or `go get -u` for this patch.
Existing imports remain unchanged; no `replace` directive is needed.

## Regression test

`TestHeartbeatPreservesApplicationWriter` opens a real WebSocket application
writer, waits for the production heartbeat, and verifies the writer remains
usable and the peer receives the intact message. It takes about 45 seconds.
The base version fails with `websocket: write closed`.

The base module omits its existing concurrent-map dependency. To preserve the
production module file, run tests using a disposable module file (shell example):

```sh
cp go.mod regression.mod
go mod edit -modfile=regression.mod -require=github.com/lixiang4u/concurrent-map@v0.0.0-20250319060614-67b87a31cef3
go test -race -mod=mod -modfile=regression.mod -count=1 -timeout=90s ./...
```

Do not commit generated regression.mod/regression.sum or consumer build artifacts.
Consumer validation must include compilation and the application's WebSocket
business flows before production deployment.
