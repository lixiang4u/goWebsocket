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

	app := fiber.New(fiber.Config{TrustProxy: true})

	app.Get("/index.html", func(ctx fiber.Ctx) error {
		return ctx.SendFile("./_examples/index.html")
	})

	app.Get("/websocket", adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println("[websocket]", time.Now().String())
		appSocket.Handler(writer, request, nil)
	}))

	app.Get("/stat", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"ListConn":  appSocket.ListConn(),
			"ListUser":  appSocket.ListUser(),
			"ListGroup": appSocket.ListGroup(),
		})
	})

	app.Get("/debug", func(ctx fiber.Ctx) error {
		appSocket.R(goWebsocket.ClientCtx{Id: "3bfb7f5715fa450094d2282d44de77ad", Socket: nil})
		appSocket.R(goWebsocket.ClientCtx{Id: "bbae4c7cb40e4594bcf70cf8b59037e9", Socket: nil})

		appSocket.BindUid("3bfb7f5715fa450094d2282d44de77ad", "BU222")
		appSocket.BindUid("bbae4c7cb40e4594bcf70cf8b59037e9", "BU222")

		return ctx.JSON(fiber.Map{
			"status":    "success",
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
			log.Println("[A]", clientId, uid)
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
