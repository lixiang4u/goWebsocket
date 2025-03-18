package main

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/lixiang4u/goWebsocket"
	"log"
	"net/http"
	"time"
)

var appSocket = goWebsocket.NewWebsocketManager()

func main() {

	app := fiber.New(fiber.Config{TrustProxy: true})

	app.Get("/index.html", func(ctx fiber.Ctx) error {
		//var _bindUid = "BU11111111"
		//appSocket.Store("5a356db8dbd746f3a0a75d24bee3d09f")
		//appSocket.Store("b0e3b7f1dd044203a82109389f8cade2")
		//
		//appSocket.BindUid("5a356db8dbd746f3a0a75d24bee3d09f", _bindUid)
		//appSocket.BindUid("b0e3b7f1dd044203a82109389f8cade2", _bindUid)
		//
		//return ctx.SendString(JsonString(appSocket.Conn))
		return ctx.SendFile("./_examples/index.html")
	})
	app.Get("/websocket", adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println("[websocket]", time.Now().String())
		appSocket.Handler(writer, request, nil)
	}))

	app.Get("/debug", func(ctx fiber.Ctx) error {
		log.Println("[time]", time.Now().String())

		var clientId = ctx.Query("client_id")
		var bindUid = ctx.Query("bind_uid")
		var unbindUid = ctx.Query("unbind_uid")
		if len(bindUid) > 0 {
			appSocket.BindUid(clientId, bindUid)
		}
		if len(unbindUid) > 0 {
			//appSocket.UnbindUid(clientId, unbindUid)
		}

		log.Println(fmt.Sprintf("%#v", appSocket.Conn.Conn))
		log.Println(fmt.Sprintf("%#v", appSocket.Conn.Uid))
		log.Println(fmt.Sprintf("%#v", appSocket.Conn.Group))
		log.Println("[appSocket.Conn.Conn]", JsonString(appSocket.Conn.Conn))

		var rsp = fiber.Map{
			"time":  time.Now().Unix(),
			"Conn":  appSocket.Conn.Conn,
			"Group": appSocket.Conn.Group,
			"Uid":   appSocket.Conn.Uid,
		}

		return RespSuccessData(&ctx, rsp)
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
