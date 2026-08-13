package goWebsocket

import (
	"sort"
	"sync"

	"github.com/gorilla/websocket"
)

type userEventTask struct {
	handler  EventHandler
	ctx      EventCtx
	terminal bool
}

type clientEventWorker struct {
	dispatcher *eventDispatcher
	clientID   string
	tasks      chan userEventTask
	done       chan struct{}
	socket     *websocket.Conn
	fromCtx    *FromCtx

	mu               sync.Mutex
	terminalAccepted bool
	terminalTask     userEventTask
	overflowOnce     sync.Once
}

type eventDispatcher struct {
	mu          sync.RWMutex
	workers     map[string]*clientEventWorker
	global      *clientEventWorker
	queueSize   int
	onPanic     func()
	onQueueFull func()
}

func newEventDispatcher(queueSize int, onPanic func(), onQueueFull ...func()) *eventDispatcher {
	if queueSize <= 0 {
		queueSize = defaultEventQueueSize
	}
	dispatcher := &eventDispatcher{
		workers:   make(map[string]*clientEventWorker),
		queueSize: queueSize,
		onPanic:   onPanic,
	}
	if len(onQueueFull) > 0 {
		dispatcher.onQueueFull = onQueueFull[0]
	}
	return dispatcher
}

func (d *eventDispatcher) replaceClient(clientID string, connection ...EventCtx) *clientEventWorker {
	worker := d.newWorker(clientID)
	if len(connection) > 0 {
		worker.socket = connection[0].Socket
		worker.fromCtx = cloneFromCtx(connection[0].FromCtx)
	}
	d.mu.Lock()
	d.workers[clientID] = worker
	d.mu.Unlock()
	return worker
}

func cloneFromCtx(source *FromCtx) *FromCtx {
	if source == nil {
		return nil
	}
	copyValue := *source
	copyValue.Groups = append([]string(nil), source.Groups...)
	return &copyValue
}

func (w *clientEventWorker) updateDynamicContext(uid string, groups []string) {
	w.mu.Lock()
	if w.fromCtx == nil {
		w.fromCtx = &FromCtx{}
	}
	w.fromCtx.Uid = uid
	w.fromCtx.Groups = append([]string(nil), groups...)
	sort.Strings(w.fromCtx.Groups)
	w.mu.Unlock()
}

func (w *clientEventWorker) snapshotFromCtx() *FromCtx {
	w.mu.Lock()
	defer w.mu.Unlock()
	return cloneFromCtx(w.fromCtx)
}

func (d *eventDispatcher) currentClient(clientID string) *clientEventWorker {
	d.mu.RLock()
	worker := d.workers[clientID]
	d.mu.RUnlock()
	return worker
}

func (d *eventDispatcher) globalWorker() *clientEventWorker {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.global == nil {
		d.global = d.newWorker("")
	}
	return d.global
}

func (d *eventDispatcher) newWorker(clientID string) *clientEventWorker {
	worker := &clientEventWorker{
		dispatcher: d,
		clientID:   clientID,
		tasks:      make(chan userEventTask, d.queueSize),
		done:       make(chan struct{}),
	}
	go worker.run()
	return worker
}

func (d *eventDispatcher) snapshotWorkers() []*clientEventWorker {
	d.mu.RLock()
	workers := make([]*clientEventWorker, 0, len(d.workers)+1)
	for _, worker := range d.workers {
		workers = append(workers, worker)
	}
	if d.global != nil {
		workers = append(workers, d.global)
	}
	d.mu.RUnlock()
	return workers
}

func (d *eventDispatcher) remove(worker *clientEventWorker) {
	d.mu.Lock()
	if worker.clientID == "" {
		if d.global == worker {
			d.global = nil
		}
	} else if d.workers[worker.clientID] == worker {
		delete(d.workers, worker.clientID)
	}
	d.mu.Unlock()
}

func (w *clientEventWorker) submit(task userEventTask) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.terminalAccepted {
		return false
	}
	if task.terminal {
		w.terminalAccepted = true
		w.terminalTask = task
		close(w.tasks)
		return true
	}
	select {
	case w.tasks <- task:
		return true
	default:
		w.overflowOnce.Do(func() {
			if w.dispatcher.onQueueFull != nil {
				w.dispatcher.onQueueFull()
			}
		})
		return false
	}
}

func (w *clientEventWorker) run() {
	defer close(w.done)
	for task := range w.tasks {
		w.execute(task)
	}
	w.mu.Lock()
	terminalTask := w.terminalTask
	w.mu.Unlock()
	w.execute(terminalTask)
	w.dispatcher.remove(w)
}

func (w *clientEventWorker) execute(task userEventTask) {
	if task.handler == nil {
		return
	}
	defer func() {
		if recover() != nil && w.dispatcher.onPanic != nil {
			w.dispatcher.onPanic()
		}
	}()
	task.handler(task.ctx)
}
