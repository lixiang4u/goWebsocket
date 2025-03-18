package goWebsocket

import (
	"github.com/gorilla/websocket"
)

type ClientMapEmpty map[string]bool

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

type ConnectionCtx struct {
	Socket *websocket.Conn `json:"socket,omitempty"`
	Group  map[string]bool `json:"group"`
	Uid    string          `json:"uid"`
}

//type DataHub struct {
//	Uid   sync.Map // [Uid => ClientMapEmpty]
//	Group sync.Map // [GroupName => ClientMapEmpty]
//	Conn  sync.Map // [ClientId => ConnectionCtx]
//}
