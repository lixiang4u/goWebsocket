package main

import (
	"fmt"
	go_websocket "github.com/lixiang4u/go-websocket"
	"log"
	"net/http"
)

var (
	addr = "127.0.0.1:8088"
)

func main() {
	var ws = go_websocket.NewWebsocketManager()
	////ws.Config.Debug = true
	////注册列表数据查询
	//ws.On("list", func(clientId string, ws *websocket.Conn, messageType int, data map[string]interface{}) bool {
	//	b, _ := json.MarshalIndent(go_websocket.WSConnectionList(), "", "	")
	//	_ = ws.WriteMessage(messageType, b)
	//	return true
	//})
	////注册广播消息
	//ws.On("broadcast", func(clientId string, ws *websocket.Conn, messageType int, data map[string]interface{}) bool {
	//	b, _ := json.MarshalIndent(data, "", "	")
	//	go_websocket.WSBroadcast(clientId, messageType, b)
	//	return true
	//})

	//ws.On(go_websocket.Event(go_websocket.EventHelp).String(), func(clientId string, ws *websocket.Conn, messageType int, data go_websocket.EventProtocol) bool {
	//
	//	log.Println("[执行了自定义事件]", clientId)
	//
	//	return true
	//})

	ws.Config.Debug = true

	http.Handle("/", http.FileServer(http.Dir("./_examples")))
	http.HandleFunc("/register", func(writer http.ResponseWriter, request *http.Request) {
		ws.Handler(writer, request, nil)
	})

	fmt.Printf("Now listening on: http://%s\r\n", addr)
	log.Fatalln(http.ListenAndServe(addr, nil))
}
