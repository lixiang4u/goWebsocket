package goWebsocket

type ConnectionCtxPlain struct {
	Group map[string]bool `json:"group"`
	Uid   string          `json:"uid"`
}

func (x *WebsocketManager) connect(ctx EventCtx, worker *clientEventWorker) {
	var writer *connectionWriter
	if ctx.Socket != nil {
		writer = newConnectionWriter(ctx.Socket, x.Config.OutboundQueueSize, writeTicker, func(reason writerFailure) {
			x.handleWriterFailure(ctx.From, worker, reason)
		})
	}
	x.connectionRecords.Set(ctx.From, connectionRecord{socket: ctx.Socket, worker: worker, writer: writer, ready: true})
	if x.beforePublishConnection != nil {
		x.beforePublishConnection(ctx.From)
	}
	x.clients.Set(ctx.From, ConnectionCtx{
		Socket: ctx.Socket,
		Group:  make(map[string]bool),
		Uid:    "",
	})
}

func (x *WebsocketManager) disconnectWorker(clientID string, worker *clientEventWorker) {
	record, exists := x.connectionRecords.Get(clientID)
	if !exists || record.worker != worker {
		return
	}
	x.clients.RemoveCb(clientID, func(key string, v ConnectionCtx, exists bool) bool {
		if !exists {
			return false
		}
		if len(v.Uid) > 0 {
			x._removeUserOp(SeqOpCtx{
				ClientId: key,
				Uid:      v.Uid,
			})
		}
		if v.Group != nil {
			for tmpGroupName, _ := range v.Group {
				x._removeGroupOp(SeqOpCtx{
					ClientId: key,
					Group:    tmpGroupName,
				})
			}
		}
		return true
	})
	x.connectionRecords.RemoveCb(clientID, func(key string, current connectionRecord, exists bool) bool {
		return exists && current.worker == worker
	})
}

func (x *WebsocketManager) bindUid(clientId, uid string) bool {
	if len(clientId) == 0 || len(uid) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	var tmpConn = v
	var prevUid = tmpConn.Uid

	tmpConn.Uid = uid
	x.clients.Set(clientId, tmpConn)

	if len(prevUid) > 0 && prevUid != uid {
		// 删除旧Uid
		x._removeUserOp(SeqOpCtx{
			ClientId: clientId,
			Uid:      prevUid,
		})
	}

	// 绑定新Uid
	x._addUserOp(SeqOpCtx{
		ClientId: clientId,
		Uid:      uid,
	})

	return true
}

func (x *WebsocketManager) unbindUid(clientId, uid string) bool {
	if len(clientId) == 0 || len(uid) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	if len(v.Uid) == 0 {
		return false
	}

	v.Uid = ""
	x.clients.Set(clientId, v)

	x._removeUserOp(SeqOpCtx{
		ClientId: clientId,
		Uid:      uid,
	})

	return true
}

func (x *WebsocketManager) joinGroup(clientId, group string) bool {
	if len(clientId) == 0 || len(group) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}
	if v.Group == nil {
		v.Group = make(map[string]bool)
	}
	if _, ok = v.Group[group]; !ok {
		v.Group[group] = true
		x.clients.Set(clientId, v)
	}

	x._addGroupOp(SeqOpCtx{
		ClientId: clientId,
		Group:    group,
	})

	return true
}

func (x *WebsocketManager) leaveGroup(clientId, group string) bool {
	if len(clientId) == 0 || len(group) == 0 {
		return false
	}
	v, ok := x.clients.Get(clientId)
	if !ok {
		return false
	}

	if v.Group != nil {
		delete(v.Group, group)
		x.clients.Set(clientId, v)
	}

	x._removeGroupOp(SeqOpCtx{
		ClientId: clientId,
		Group:    group,
	})

	return true
}

func (x *WebsocketManager) _send(clientId string, messageType int, data interface{}) bool {
	payload, ok := x.marshalOutbound(data)
	if !ok {
		return false
	}
	result := x.enqueueRecipients(x.snapshotClientRecipients([]string{clientId}), messageType, payload)
	return len(result.Accepted) == 1
}

func (x *WebsocketManager) enqueueClient(clientID string, worker *clientEventWorker, messageType int, payload []byte) bool {
	record, ok := x.connectionRecords.Get(clientID)
	if !ok || !record.ready || record.writer == nil || (worker != nil && record.worker != worker) {
		return false
	}
	connection, published := x.clients.Get(clientID)
	if !published || connection.Socket != record.socket {
		return false
	}
	return record.writer.enqueue(messageType, payload)
}
