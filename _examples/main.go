package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/lixiang4u/goWebsocket"
	"log"
	"time"
)

var appSocket = goWebsocket.NewWebsocketManager()

func main() {
	app := fiber.New(fiber.Config{
		Immutable:        true,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true, LinkLocal: true, Private: true},
	})

	app.Use(cors.New())

	app.Get("/debug.html", func(ctx fiber.Ctx) error {
		return ctx.SendFile("./_examples/debug.html")
	})
	app.Get("/vue.global.js", func(ctx fiber.Ctx) error {
		return ctx.SendFile("./_examples/vue.global.js")
	})

	app.Get("/websocket", adaptor.HTTPHandlerFunc(HttpConn))

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
