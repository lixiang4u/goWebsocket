package goWebsocket

import "context"

import cmap "github.com/lixiang4u/concurrent-map"

const (
	defaultReadLimitBytes    int64 = 64 << 10
	defaultOutboundQueueSize       = 64
	defaultEventQueueSize          = 64
)

// ManagerConfig configures a WebsocketManager instance.
type ManagerConfig struct {
	Debug             bool
	LogPayload        bool
	ReadLimitBytes    int64
	OutboundQueueSize int
	EventQueueSize    int
}

func normalizeManagerConfig(config ManagerConfig) ManagerConfig {
	if config.ReadLimitBytes <= 0 {
		config.ReadLimitBytes = defaultReadLimitBytes
	}
	if config.OutboundQueueSize <= 0 {
		config.OutboundQueueSize = defaultOutboundQueueSize
	}
	if config.EventQueueSize <= 0 {
		config.EventQueueSize = defaultEventQueueSize
	}
	return config
}

// NewWebsocketManagerWithConfig creates a manager using normalized configuration values.
func NewWebsocketManagerWithConfig(config ManagerConfig) *WebsocketManager {
	config = normalizeManagerConfig(config)
	managerContext, cancel := context.WithCancel(context.Background())
	manager := &WebsocketManager{
		Config:       config,
		context:      managerContext,
		cancel:       cancel,
		done:         make(chan struct{}),
		registerDone: make(chan struct{}),
		shutdown:     make(chan chan managerShutdownResources, 1),
	}

	manager.register = make(chan EventCtx)
	manager.registerSync = make(chan connectionRegistration)
	manager.unregisterSync = make(chan connectionUnregistration)
	manager.unregister = make(chan connectionUnregistration)
	manager.bind = make(chan EventCtx)
	manager.unbind = make(chan EventCtx)
	manager.join = make(chan EventCtx)
	manager.leave = make(chan EventCtx)
	manager.send = make(chan EventCtx)
	manager.sendToGroup = make(chan EventCtx)
	manager.sendToUid = make(chan EventCtx)
	manager.broadcast = make(chan EventCtx)
	manager.sendSync = make(chan sendRequest)
	manager.sendGroupSync = make(chan sendRequest)
	manager.sendUidSync = make(chan sendRequest)
	manager.broadcastSync = make(chan sendRequest)

	manager.clients = cmap.New[ConnectionCtx]()
	manager.connectionRecords = cmap.New[connectionRecord]()
	manager.users = cmap.New[cmap.ConcurrentMap[string, bool]]()
	manager.groups = cmap.New[cmap.ConcurrentMap[string, bool]]()
	manager.eventDispatcher = newEventDispatcher(config.EventQueueSize, func() {
		manager.Log("[WebsocketEventHandlerPanic]")
	}, func() {
		manager.Log("[WebsocketEventQueueFull]")
	})

	go manager.registerEvent()

	return manager
}

// NewWebsocketManager preserves the legacy debug-only constructor.
func NewWebsocketManager(debug ...bool) *WebsocketManager {
	config := ManagerConfig{}
	if len(debug) > 0 {
		config.Debug = debug[0]
	}
	return NewWebsocketManagerWithConfig(config)
}
