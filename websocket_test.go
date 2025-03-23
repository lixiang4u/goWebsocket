package goWebsocket

import (
	"testing"
)

//var socket = NewWebsocketManager()
//
//var maxLength = 10000000
//var clients = make([]string, maxLength)
//
//func init() {
//	for i := 0; i < maxLength; i++ {
//		id := UUID()
//		clients = append(clients, id)
//		socket.Register(EventCtx{
//			From:   id,
//			Socket: nil,
//		})
//	}
//}

func MM() {
	var mm = make(map[string]map[string]bool)

	if v, ok := mm["sd"]; !ok {
		if v == nil {
			v = make(map[string]bool)
		}
		v["SSD"] = true
		mm["sd"] = v
	}
}

func BenchmarkMM(b *testing.B) {
	for i := 0; i < b.N; i++ {
		MM()
	}

}

func BenchmarkBindUid(b *testing.B) {
	var socket = NewWebsocketManager()
	for i := 0; i < b.N; i++ {
		socket.BindUid(UUID(), UUID(7))
	}
}

func BenchmarkJoinGroup(b *testing.B) {
	var socket = NewWebsocketManager()
	for i := 0; i < b.N; i++ {
		socket.JoinGroup(UUID(), UUID(7))
	}
}

func BenchmarkUnbindUid(b *testing.B) {
	var socket = NewWebsocketManager()
	for i := 0; i < b.N; i++ {
		socket.UnbindUid(UUID(), UUID(7))
	}
}

func BenchmarkLeaveGroup(b *testing.B) {
	var socket = NewWebsocketManager()
	for i := 0; i < b.N; i++ {
		socket.LeaveGroup(UUID(), UUID(7))
	}
}

func BenchmarkBindUidPB(b *testing.B) {
	var socket = NewWebsocketManager()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket.BindUid(UUID(), UUID(7))
		}
	})
}

func BenchmarkJoinGroupPB(b *testing.B) {
	var socket = NewWebsocketManager()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket.JoinGroup(UUID(), UUID(7))
		}
	})
}

func BenchmarkUnbindUidPB(b *testing.B) {
	var socket = NewWebsocketManager()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket.UnbindUid(UUID(), UUID(7))
		}
	})
}

func BenchmarkLeaveGroupPB(b *testing.B) {
	var socket = NewWebsocketManager()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket.LeaveGroup(UUID(), UUID(7))
		}
	})
}
