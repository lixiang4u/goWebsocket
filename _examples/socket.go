package main

import (
	"github.com/gofiber/fiber/v3"
	"github.com/lixiang4u/goWebsocket"
	"github.com/mitchellh/mapstructure"
	"log"
	"net/http"
	"sync"
)

var AppSocket = goWebsocket.NewWebsocketManager()

func init() {
	addEventHandler()
}

func HttpConn(w http.ResponseWriter, r *http.Request) {
	AppSocket.Handler(w, r, nil)
}

func bindHandler(ctx goWebsocket.EventCtx) bool {
	type Req struct {
		Token  string `mapstructure:"token"`
		UserId string `mapstructure:"user_id"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[bindHandler] event=bind decode_error")
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 500, "msg": err.Error()},
		})
		return false
	}
	//claims, ok := helper.VerifyJwtToken(req.Token)
	//if !ok {
	//	AppSocket.Send(ctx.From, goWebsocket.EventCtx{
	//		Event: ctx.Event,
	//		Data:  fiber.Map{"code": 500, "msg": "认证失败"},
	//	})
	//	return false
	//}
	var loginUserId = req.UserId
	AppSocket.BindUid(ctx.From, loginUserId)
	AppSocket.Send(ctx.From, goWebsocket.EventCtx{
		Event: ctx.Event,
		Data:  fiber.Map{"code": 200, "client_id": ctx.From, "uid": loginUserId},
	})

	//user.UserAttr{}.UpdateUserOnline(cast.ToUint(claims["uid"]), 1)

	{
		ctx.Data = nil
		log.Println("[bindHandler] event=bind")
	}

	return true
}

func unbindHandler(ctx goWebsocket.EventCtx) bool {
	var uid = AppSocket.GetClientUid(ctx.From)
	if len(uid) > 0 {
		AppSocket.UnbindUid(ctx.From, uid)
		//user.UserAttr{}.UpdateUserOnline(cast.ToUint(uid), 0)
	}

	log.Println("[unbindHandler] event=unbind")

	return true
}

func connectHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[connectHandler] event=connect")

	return true
}

func disconnectHandler(ctx goWebsocket.EventCtx) bool {
	if len(ctx.FromCtx.Uid) > 0 {
		//user.UserAttr{}.UpdateUserOnline(cast.ToUint(ctx.FromCtx.Uid), 0)
	}

	log.Println("[disconnectHandler] event=disconnect")

	return true
}

func textHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[textHandler] event=text")
	type Req struct {
		ToUid   string `mapstructure:"to_uid"`
		Data    string `mapstructure:"data"`
		Context string `mapstructure:"context"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[textHandler] event=text decode_error")
		return false
	}

	return true
}

func imageHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[imageHandler] event=image")
	return true
}

func joinHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[joinHandler] event=join_group")

	type Req struct {
		GroupName string `mapstructure:"group_name"`
		Broadcast bool   `mapstructure:"broadcast"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[joinHandler] event=join_group decode_error")
		return false
	}
	if len(req.GroupName) == 0 {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 500, "msg": "分组名不能为空"},
		})
		log.Println("[joinHandler] event=join_group invalid_group")
		return false
	}
	if len(AppSocket.GetClientGroupList(ctx.From)) >= 20 {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 500, "msg": "已加入分组数量超过20"},
		})
		log.Println("[joinHandler] event=join_group group_limit_reached")
		return false
	}

	log.Println("[joinHandler] event=join_group accepted")
	AppSocket.JoinGroup(ctx.From, req.GroupName)

	log.Println("[joinHandler] event=join_group connection_snapshot_updated")

	if req.Broadcast {
		AppSocket.SendToGroup(req.GroupName, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 200, "msg": "已加入分组"},
		})
	} else {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 200, "msg": "已加入分组"},
		})
	}

	return true
}

func leaveHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[leaveHandler] event=leave_group")

	type Req struct {
		GroupName string `mapstructure:"group_name"`
		Broadcast bool   `mapstructure:"broadcast"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[leaveHandler] event=leave_group decode_error")
		return false
	}
	if len(req.GroupName) == 0 {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 500, "msg": "分组名不能为空"},
		})
		return false
	}
	if req.Broadcast {
		AppSocket.SendToGroup(req.GroupName, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 200, "msg": "已退出分组"},
		})
	} else {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 200, "msg": "已退出分组"},
		})
	}
	AppSocket.LeaveGroup(ctx.From, req.GroupName)

	return true
}

func groupBroadcastHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[groupBroadcastHandler] event=group_broadcast")

	type Req struct {
		GroupName string      `mapstructure:"group_name"`
		Data      interface{} `mapstructure:"data"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[groupBroadcastHandler] event=group_broadcast decode_error")
		return false
	}
	if len(req.GroupName) == 0 {
		AppSocket.Send(ctx.From, goWebsocket.EventCtx{
			Event: ctx.Event,
			Data:  fiber.Map{"code": 500, "msg": "分组名不能为空"},
		})
		return false
	}
	log.Println("[groupBroadcastHandler] event=group_broadcast recipients_updated")
	AppSocket.SendToGroup(req.GroupName, req.Data)

	return true

}

func shopHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[shopHandler] event=shop")
	return true
}

func clickHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[clickHandler] event=click")
	return true
}

func onlineHandler(ctx goWebsocket.EventCtx) bool {
	log.Println("[onlineHandler] event=online")
	type Req struct {
		Uid string `mapstructure:"uid"`
	}
	var req Req
	if err := mapstructure.Decode(ctx.Data, &req); err != nil {
		log.Println("[onlineHandler] event=online decode_error")
		return false
	}

	AppSocket.Send(ctx.From, goWebsocket.EventCtx{
		Event: ctx.Event,
		Data:  fiber.Map{"code": 200, "online": AppSocket.UidClientCount(req.Uid)},
	})

	return true
}

func addEventHandler() {
	sync.OnceFunc(func() {
		// 事件名对应前端发送的消息事件名
		AppSocket.On("bind", bindHandler)
		AppSocket.On("unbind", unbindHandler)
		AppSocket.On(goWebsocket.Event(goWebsocket.EventConnect).String(), connectHandler)
		AppSocket.On(goWebsocket.Event(goWebsocket.EventClose).String(), disconnectHandler)
		//AppSocket.On("join_room", bindEvent)
		//AppSocket.On("leaveGroup", bindEvent)
		AppSocket.On("online", onlineHandler)
		AppSocket.On("text", textHandler)
		AppSocket.On("image", imageHandler)
		AppSocket.On("join_group", joinHandler)
		AppSocket.On("leave_group", leaveHandler)
		AppSocket.On("group_broadcast", groupBroadcastHandler)

		AppSocket.On("shop", shopHandler)

		AppSocket.On("click", clickHandler)
	})()
}
