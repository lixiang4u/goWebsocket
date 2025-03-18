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

	app.Get("/bind-uid", func(ctx fiber.Ctx) error {
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

	app.Get("/debug", func(ctx fiber.Ctx) error {

		var clientId = ctx.Query("client_id")
		var bindUid = ctx.Query("bind_uid")
		var unbindUid = ctx.Query("unbind_uid")
		if len(bindUid) > 0 {
			log.Println("[BindUid]", clientId, "=>", bindUid)
		}
		if len(unbindUid) > 0 {
			//appSocket.UnbindUid(clientId, unbindUid)
		}

		//log.Println(fmt.Sprintf("%#v", appSocket.Conn.Uid))
		//log.Println(fmt.Sprintf("%#v", appSocket.Conn.Group))

		// 再次检测是否有相同map键
		//var tmpMap = make(map[string]bool)
		//for tmpClientId, _ := range appSocket.Conn.Conn {
		//	if _, ok := tmpMap[tmpClientId]; !ok {
		//		tmpMap[tmpClientId] = true
		//	} else {
		//		log.Println("[已经存在Key]", tmpClientId)
		//	}
		//}

		var rsp = fiber.Map{
			"time": time.Now().Unix(),
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
