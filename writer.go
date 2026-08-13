package goWebsocket

import (
	"sync"
	"time"
)

type messageWriter interface {
	SetWriteDeadline(time.Time) error
	WriteMessage(int, []byte) error
}

type outboundFrame struct {
	messageType int
	payload     []byte
}

type writerFailure uint8

const (
	writerFailureQueueFull writerFailure = iota + 1
	writerFailureWrite
)

type connectionWriter struct {
	socket        messageWriter
	outbound      chan outboundFrame
	stopCh        chan struct{}
	done          chan struct{}
	heartbeatDone chan struct{}
	onFailure     func(writerFailure)

	mu       sync.Mutex
	stopped  bool
	failOnce sync.Once

	afterQueueFullObserved func()
}

func newConnectionWriter(socket messageWriter, queueSize int, pingInterval time.Duration, onFailure func(writerFailure)) *connectionWriter {
	if queueSize <= 0 {
		queueSize = defaultOutboundQueueSize
	}
	writer := &connectionWriter{
		socket:        socket,
		outbound:      make(chan outboundFrame, queueSize),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		heartbeatDone: make(chan struct{}),
		onFailure:     onFailure,
	}
	if pingInterval > 0 {
		go writer.runHeartbeat(pingInterval)
	} else {
		close(writer.heartbeatDone)
	}
	go writer.run()
	return writer
}

func (w *connectionWriter) enqueue(messageType int, payload []byte) bool {
	frame := outboundFrame{
		messageType: messageType,
		payload:     append([]byte{}, payload...),
	}
	w.mu.Lock()
	if w.stopped {
		w.mu.Unlock()
		return false
	}
	select {
	case w.outbound <- frame:
		w.mu.Unlock()
		return true
	default:
		transitioned := w.stopLocked()
		w.mu.Unlock()
		if w.afterQueueFullObserved != nil {
			w.afterQueueFullObserved()
		}
		if transitioned {
			w.notifyFailure(writerFailureQueueFull)
		}
		return false
	}
}

func (w *connectionWriter) stop() {
	w.mu.Lock()
	w.stopLocked()
	w.mu.Unlock()
}

func (w *connectionWriter) fail(reason writerFailure) {
	w.mu.Lock()
	transitioned := w.stopLocked()
	w.mu.Unlock()
	if transitioned {
		w.notifyFailure(reason)
	}
}

func (w *connectionWriter) stopLocked() bool {
	if w.stopped {
		return false
	}
	w.stopped = true
	close(w.stopCh)
	return true
}

func (w *connectionWriter) notifyFailure(reason writerFailure) {
	w.failOnce.Do(func() {
		if w.onFailure != nil {
			w.onFailure(reason)
		}
	})
}

func (w *connectionWriter) run() {
	defer func() {
		<-w.heartbeatDone
		close(w.done)
	}()
	for {
		select {
		case <-w.stopCh:
			return
		default:
		}
		select {
		case <-w.stopCh:
			return
		case frame := <-w.outbound:
			if w.isStopped() {
				return
			}
			if err := w.socket.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				w.fail(writerFailureWrite)
				return
			}
			if err := w.socket.WriteMessage(frame.messageType, frame.payload); err != nil {
				w.fail(writerFailureWrite)
				return
			}
		}
	}
}

func (w *connectionWriter) isStopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func (w *connectionWriter) runHeartbeat(interval time.Duration) {
	defer close(w.heartbeatDone)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			if !w.enqueue(pingMessageType, nil) {
				return
			}
		}
	}
}
