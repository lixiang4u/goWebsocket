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

	app.Get("/bind-uid-v1", func(ctx fiber.Ctx) error {
		var clientId = ctx.Query("client_id")
		var uid = ctx.Query("uid")

		if len(clientId) > 0 && len(uid) > 0 {
			appSocket.BindUid(clientId, uid)
		}

		return ctx.JSON(fiber.Map{
			"status":    "success",
			"ListConn":  appSocket.ListConn(),
			"ListUser":  appSocket.ListUser(),
			"ListGroup": appSocket.ListGroup(),
		})
	})

	app.Get("/bind-uid-v2", func(ctx fiber.Ctx) error {
		var clientId = ctx.Query("client_id")
		var uid = ctx.Query("uid")

		if len(clientId) > 0 && len(uid) > 0 {
			log.Println("[bind-uid-v2]", clientId, uid)
		}

		return ctx.JSON(fiber.Map{
			"status":    "success",
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
