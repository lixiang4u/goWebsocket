package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/lixiang4u/goWebsocket"
	"log"
	"net/http"
	"time"
)

var appSocket = goWebsocket.NewWebsocketManager()

func main() {

	//var app = gin.New()
	//app.Use(gin.Logger(), gin.Recovery())
	//
	//app.StaticFile("/", filepath.Join(goWebsocket.AppPath(), "_examples/index.html"))
	//
	//app.GET("websocket", func(ctx *gin.Context) {
	//	log.Println("[websocket]", time.Now().String())
	//	appSocket.Handler(ctx.Writer, ctx.Request, nil)
	//})
	//
	//app.GET("/stat", func(ctx *gin.Context) {
	//	ctx.JSON(200, gin.H{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//
	//})
	//
	//app.GET("bind-uid-v1", func(ctx *gin.Context) {
	//	var clientId = ctx.Query("client_id")
	//	var uid = ctx.Query("uid")
	//	if len(clientId) > 0 && len(uid) > 0 {
	//		appSocket.BindUid(clientId, uid)
	//	}
	//	ctx.JSON(200, gin.H{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//})
	//
	//app.GET("bind-uid-v2", func(ctx *gin.Context) {
	//	var clientId = ctx.Query("client_id")
	//	var uid = ctx.Query("uid")
	//	if len(clientId) > 0 && len(uid) > 0 {
	//		log.Println("[V2]")
	//	}
	//	ctx.JSON(200, gin.H{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//})
	//
	//_ = app.Run(":10800")

	//app := iris.New()
	//
	//app.Get("index.html", func(ctx *context.Context) {
	//	_ = ctx.ServeFile("./_examples/index.html")
	//})
	//
	//app.Get("websocket", func(ctx *context.Context) {
	//	log.Println("[websocket]", time.Now().String())
	//	appSocket.Handler(ctx.ResponseWriter(), ctx.Request(), nil)
	//})
	//
	//app.Get("/stat", func(ctx *context.Context) {
	//	_ = ctx.JSON(iris.Map{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//})
	//
	//app.Get("bind-uid-v1", func(ctx *context.Context) {
	//	var clientId = ctx.URLParam("client_id")
	//	var uid = ctx.URLParam("uid")
	//	if len(clientId) > 0 && len(uid) > 0 {
	//		appSocket.BindUid(clientId, uid)
	//	}
	//	_ = ctx.JSON(iris.Map{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//})
	//
	//app.Get("bind-uid-v2", func(ctx *context.Context) {
	//	var clientId = ctx.URLParam("client_id")
	//	var uid = ctx.URLParam("uid")
	//	if len(clientId) > 0 && len(uid) > 0 {
	//		log.Println("[V2]")
	//	}
	//	_ = ctx.JSON(iris.Map{
	//		"ListConn":  appSocket.ListConn(),
	//		"ListUser":  appSocket.ListUser(),
	//		"ListGroup": appSocket.ListGroup(),
	//	})
	//})
	//
	//_ = app.Listen(":10800")

	// EventConnect EventClose EventBindUid EventUnbindUid EventJoinGroup EventLeaveGroup EventSendToClient EventSendToGroup EventSendToUid EventBroadcast

	appSocket.On(goWebsocket.Event(goWebsocket.EventConnect).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventConnect]", clientId, data)
		appSocket.Send(clientId, fiber.Map{"clientId": clientId})
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventClose).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventClose]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventBindUid).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventBindUid]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventUnbindUid).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventUnbindUid]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventJoinGroup).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventJoinGroup]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventLeaveGroup).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventLeaveGroup]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventSendToClient).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventSendToClient]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventSendToGroup).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventSendToGroup]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventSendToUid).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventSendToUid]", clientId, data)
		return true
	})
	appSocket.On(goWebsocket.Event(goWebsocket.EventBroadcast).String(), func(clientId string, data goWebsocket.EventCtx) bool {
		log.Println("[EventBroadcast]", clientId, data)
		return true
	})

	app := fiber.New(fiber.Config{Immutable: true})

	app.Get("/index.html", func(ctx fiber.Ctx) error {
		return ctx.SendFile("./_examples/index.html")
	})

	app.Get("/websocket", adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		appSocket.Handler(writer, request, nil)
	}))

	app.Get("/stat", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"ListConn":  appSocket.ListConn(),
			"ListUser":  appSocket.ListUser(),
			"ListGroup": appSocket.ListGroup(),
		})
	})

	app.Get("/debug-socket", func(ctx fiber.Ctx) error {
		var clientId = ctx.Query("client_id")
		var uid = ctx.Query("uid")
		var group = ctx.Query("group")
		var event = ctx.Query("event")

		if len(clientId) > 0 {
			switch event {
			case "bind":
				appSocket.BindUid(clientId, uid)
				break
			case "unbind":
				appSocket.UnbindUid(clientId, uid)
				break
			case "join":
				appSocket.JoinGroup(clientId, group)
				break
			case "leave":
				appSocket.LeaveGroup(clientId, group)
				break
			case "send":
				appSocket.Send(clientId, fiber.Map{
					"to":   clientId,
					"time": time.Now().String(),
					"r":    goWebsocket.UUID(),
				})
				break
			case "send-group":
				appSocket.SendToGroup(group, fiber.Map{
					"to":   "send-group-" + group,
					"time": time.Now().String(),
					"r":    goWebsocket.UUID(),
				})
				break
			case "send-uid":
				appSocket.SendToUid(uid, fiber.Map{
					"to":   "send-uid-" + uid,
					"time": time.Now().String(),
					"r":    goWebsocket.UUID(),
				})
				break
			case "broadcast":
				appSocket.SendToAll(fiber.Map{
					"to":   "broadcast",
					"time": time.Now().String(),
					"r":    goWebsocket.UUID(),
				})
				break
			}
		}

		return ctx.JSON(fiber.Map{
			"status":    "success",
			"ListConn":  appSocket.ListConn(),
			"ListUser":  appSocket.ListUser(),
			"ListGroup": appSocket.ListGroup(),
		})
	})

	app.Get("/debug-xx1", func(ctx fiber.Ctx) error {
		var x []string
		for tmpId, _ := range appSocket.ListConn() {
			x = append(x, tmpId)
		}

		log.Println("[x]", goWebsocket.ToJson(x))

		for idx, tmpId := range x {
			switch idx {
			case 0:
				appSocket.BindUid(tmpId, "u1001")
				appSocket.JoinGroup(tmpId, "g0008")
			case 1:
				appSocket.BindUid(tmpId, "u1002")
				appSocket.JoinGroup(tmpId, "g0008")
			case 2:
				appSocket.BindUid(tmpId, "u1003")
				appSocket.JoinGroup(tmpId, "g0007")
			case 3:
				appSocket.JoinGroup(tmpId, "g0009")
			case 4:
				appSocket.JoinGroup(tmpId, "g0008")
				appSocket.JoinGroup(tmpId, "g0009")
			case 5:
			}
		}

		return ctx.JSON(fiber.Map{
			"ListConn":  appSocket.ListConn(),
			"ListUser":  appSocket.ListUser(),
			"ListGroup": appSocket.ListGroup(),
		})
	})

	log.Fatal(app.Listen(":10800"))

}

func JsonString(v any) string {
	buff, err := json.Marshal(v)
	//buff, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return ""
	}
	return string(buff)
}
func NowUnixTime() int64 {
	return time.Now().Unix()
}

func RespSuccessData(ctx *fiber.Ctx, data fiber.Map) error {
	return (*ctx).JSON(SuccessResp(fiber.Map{"data": data}))
}
func SuccessResp(data fiber.Map) fiber.Map {
	data["code"] = 200
	return data
}
