package goWebsocket

import (
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
)

//type MapKB cmap.ConcurrentMap[string, bool]

type MapClientCtx cmap.ConcurrentMap[string, ConnectionCtx]

type ConnectionCtx struct {
	Socket *websocket.Conn                                              `json:"socket,omitempty"`
	Group  cmap.ConcurrentMap[string, cmap.ConcurrentMap[string, bool]] `json:"group"`
	Uid    string                                                       `json:"uid"`
}

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

type ClientCtx struct {
	Id     string // 客户端Id
	Socket *websocket.Conn
}

type MessageCtx struct {
	Id  string // 客户端Id
	Msg []byte
}

type CmdCtx struct {
	Id   string // 客户端Id
	Cmd  int
	Data any
}

//type DataHub struct {
//	Uid   sync.Map // [Uid => ClientMapEmpty]
//	Group sync.Map // [GroupName => ClientMapEmpty]
//	Conn  sync.Map // [ClientId => ConnectionCtx]
//}
