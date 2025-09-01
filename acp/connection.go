package acp

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMaxRetries         = 10
	defaultMaxDelaySeconds    = 30
	defaultBackoffFactor      = 2.0
	defaultJitterFactor       = 0.1
	defaultShutdownTimeoutSec = 30
	shutdownTimeoutSec        = 5
	randomRange               = 1000
	stackDepth                = 32
)

// ConnectionEvent represents a connection lifecycle event.
type ConnectionEvent int

const (
	// ConnectionEventConnected indicates a connection was established.
	ConnectionEventConnected ConnectionEvent = iota
	// ConnectionEventDisconnected indicates a connection was closed.
	ConnectionEventDisconnected
	// ConnectionEventError indicates an error occurred.
	ConnectionEventError
	// ConnectionEventReconnecting indicates a reconnection attempt.
	ConnectionEventReconnecting
	// ConnectionEventStateChanged indicates the connection state changed.
	ConnectionEventStateChanged
)

// String returns the string representation of the connection event.
func (ce ConnectionEvent) String() string {
	switch ce {
	case ConnectionEventConnected:
		return "connected"
	case ConnectionEventDisconnected:
		return "disconnected"
	case ConnectionEventError:
		return "error"
	case ConnectionEventReconnecting:
		return "reconnecting"
	case ConnectionEventStateChanged:
		return "state_changed"
	default:
		return "unknown"
	}
}

// ConnectionEventData contains data associated with a connection event.
type ConnectionEventData struct {
	Event     ConnectionEvent
	Timestamp time.Time
	Error     error
	Metadata  map[string]interface{}
}

// ConnectionEventHandler handles connection events.
type ConnectionEventHandler func(event ConnectionEvent, data interface{})

// ManagedConnection provides enhanced connection management with lifecycle handling.
type ManagedConnection struct {
	// Underlying connection (either AgentConnection or ClientConnection)
	underlying interface{}

	// Connection state
	state   ConnectionState
	stateMu sync.RWMutex

	// Event handlers
	handlers   []ConnectionEventHandler
	handlersMu sync.RWMutex

	// Reconnection settings
	reconnectConfig *ReconnectConfig
	reconnectMu     sync.RWMutex

	// Graceful shutdown
	shutdownTimeout time.Duration
	shutdownCh      chan struct{}
	shutdownOnce    sync.Once

	// Metrics and monitoring
	connectedAt    time.Time
	disconnectedAt time.Time
	messageCount   atomic.Int64
	errorCount     atomic.Int64

	// Session manager
	sessionManager *SessionManager

	// Terminal manager (for client connections)
	terminalManager *SessionTerminalManager

	// Context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
}

// ReconnectConfig configures reconnection behavior.
type ReconnectConfig struct {
	Enabled         bool
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	JitterFactor    float64
	OnReconnect     func(attempt int)
	OnReconnectFail func(err error)
}

// DefaultReconnectConfig returns a default reconnection configuration.
func DefaultReconnectConfig() *ReconnectConfig {
	return &ReconnectConfig{
		Enabled:       true,
		MaxRetries:    defaultMaxRetries,
		InitialDelay:  time.Second,
		MaxDelay:      defaultMaxDelaySeconds * time.Second,
		BackoffFactor: defaultBackoffFactor,
		JitterFactor:  defaultJitterFactor,
	}
}

// NewManagedConnection creates a new managed connection.
func NewManagedConnection(conn interface{}) (*ManagedConnection, error) {
	// Validate connection type
	switch conn.(type) {
	case *AgentConnection, *ClientConnection:
		// Valid connection types
	default:
		return nil, fmt.Errorf("invalid connection type: %T", conn)
	}

	ctx, cancel := context.WithCancel(context.Background())

	mc := &ManagedConnection{
		underlying:      conn,
		state:           StateUninitialized,
		handlers:        make([]ConnectionEventHandler, 0),
		shutdownTimeout: defaultShutdownTimeoutSec * time.Second,
		shutdownCh:      make(chan struct{}),
		connectedAt:     time.Now(),
		sessionManager:  NewSessionManager(),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Initialize terminal manager for client connections
	if clientConn, ok := conn.(*ClientConnection); ok {
		mc.terminalManager = NewSessionTerminalManager(clientConn)
	}

	return mc, nil
}

// OnEvent registers an event handler.
func (mc *ManagedConnection) OnEvent(handler ConnectionEventHandler) {
	mc.handlersMu.Lock()
	defer mc.handlersMu.Unlock()
	mc.handlers = append(mc.handlers, handler)
}

// RemoveAllEventHandlers removes all event handlers.
func (mc *ManagedConnection) RemoveAllEventHandlers() {
	mc.handlersMu.Lock()
	defer mc.handlersMu.Unlock()
	mc.handlers = make([]ConnectionEventHandler, 0)
}

// emitEvent emits an event to all registered handlers.
func (mc *ManagedConnection) emitEvent(event ConnectionEvent, data interface{}) {
	mc.handlersMu.RLock()
	handlers := make([]ConnectionEventHandler, len(mc.handlers))
	copy(handlers, mc.handlers)
	mc.handlersMu.RUnlock()

	eventData := ConnectionEventData{
		Event:     event,
		Timestamp: time.Now(),
	}

	// Add error if provided
	if err, ok := data.(error); ok {
		eventData.Error = err
		mc.errorCount.Add(1)
	}

	// Add metadata if provided
	if metadata, ok := data.(map[string]interface{}); ok {
		eventData.Metadata = metadata
	}

	// Call handlers asynchronously
	for _, handler := range handlers {
		go func(h ConnectionEventHandler) {
			defer func() {
				if r := recover(); r != nil {
					// Prevent panic in event handler from crashing the connection
					// Note: In production, this should use a proper logger
					_ = r
				}
			}()
			h(event, eventData)
		}(handler)
	}
}

// State management

// GetState returns the current connection state.
func (mc *ManagedConnection) GetState() ConnectionState {
	mc.stateMu.RLock()
	defer mc.stateMu.RUnlock()
	return mc.state
}

// SetState sets the connection state.
func (mc *ManagedConnection) SetState(state ConnectionState) {
	mc.stateMu.Lock()
	oldState := mc.state
	mc.state = state
	mc.stateMu.Unlock()

	if oldState != state {
		mc.emitEvent(ConnectionEventStateChanged, map[string]interface{}{
			"oldState": oldState,
			"newState": state,
		})
	}
}

// IsConnected returns true if the connection is in a connected state.
func (mc *ManagedConnection) IsConnected() bool {
	state := mc.GetState()
	return state == StateInitialized || state == StateAuthenticated || state == StateSessionReady
}

// Reconnection management

// EnableReconnect enables automatic reconnection with the given configuration.
func (mc *ManagedConnection) EnableReconnect(config *ReconnectConfig) {
	mc.reconnectMu.Lock()
	defer mc.reconnectMu.Unlock()

	if config == nil {
		config = DefaultReconnectConfig()
	}
	mc.reconnectConfig = config
}

// DisableReconnect disables automatic reconnection.
func (mc *ManagedConnection) DisableReconnect() {
	mc.reconnectMu.Lock()
	defer mc.reconnectMu.Unlock()
	mc.reconnectConfig = nil
}

// Graceful shutdown

// Shutdown performs a graceful shutdown of the connection.
func (mc *ManagedConnection) Shutdown(ctx context.Context) error {
	var err error
	mc.shutdownOnce.Do(func() {
		// Signal shutdown
		close(mc.shutdownCh)
		mc.emitEvent(ConnectionEventDisconnected, map[string]interface{}{
			"reason": "shutdown",
		})

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(ctx, mc.shutdownTimeout)
		defer cancel()

		// Release all resources
		err = mc.releaseResources(shutdownCtx)

		// Cancel the connection context
		mc.cancel()

		// Close underlying connection
		switch conn := mc.underlying.(type) {
		case *AgentConnection:
			if closeErr := conn.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		case *ClientConnection:
			if closeErr := conn.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}
	})

	return err
}

// releaseResources releases all managed resources.
func (mc *ManagedConnection) releaseResources(ctx context.Context) error {
	var errs []error

	// Release terminal handles if this is a client connection
	if mc.terminalManager != nil {
		if err := mc.terminalManager.ReleaseAll(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to release terminals: %w", err))
		}
	}

	// Clean up sessions
	for _, session := range mc.sessionManager.ListSessions() {
		if err := mc.sessionManager.DeleteSession(session.ID); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete session %v: %w", session.ID, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during resource cleanup: %v", errs)
	}

	return nil
}

// SetShutdownTimeout sets the timeout for graceful shutdown.
func (mc *ManagedConnection) SetShutdownTimeout(timeout time.Duration) {
	mc.shutdownTimeout = timeout
}

// Metrics and monitoring

// GetMetrics returns connection metrics.
func (mc *ManagedConnection) GetMetrics() ConnectionMetrics {
	uptime := time.Since(mc.connectedAt)
	if !mc.IsConnected() && !mc.disconnectedAt.IsZero() {
		uptime = mc.disconnectedAt.Sub(mc.connectedAt)
	}

	return ConnectionMetrics{
		ConnectedAt:    mc.connectedAt,
		DisconnectedAt: mc.disconnectedAt,
		Uptime:         uptime,
		MessageCount:   mc.messageCount.Load(),
		ErrorCount:     mc.errorCount.Load(),
		State:          mc.GetState(),
	}
}

// IncrementMessageCount increments the message counter.
func (mc *ManagedConnection) IncrementMessageCount() {
	mc.messageCount.Add(1)
}

// ConnectionMetrics contains connection metrics.
type ConnectionMetrics struct {
	ConnectedAt    time.Time
	DisconnectedAt time.Time
	Uptime         time.Duration
	MessageCount   int64
	ErrorCount     int64
	State          ConnectionState
}

// Session management

// GetSessionManager returns the session manager.
func (mc *ManagedConnection) GetSessionManager() *SessionManager {
	return mc.sessionManager
}

// GetTerminalManager returns the terminal manager (client connections only).
func (mc *ManagedConnection) GetTerminalManager() *SessionTerminalManager {
	return mc.terminalManager
}

// Connection pool management

// ConnectionPool manages multiple connections.
type ConnectionPool struct {
	connections map[string]*ManagedConnection
	mu          sync.RWMutex
	maxSize     int

	// Metrics
	totalCreated   atomic.Int64
	totalDestroyed atomic.Int64
}

// NewConnectionPool creates a new connection pool.
func NewConnectionPool(maxSize int) *ConnectionPool {
	return &ConnectionPool{
		connections: make(map[string]*ManagedConnection),
		maxSize:     maxSize,
	}
}

// Get retrieves a connection by ID.
func (cp *ConnectionPool) Get(id string) (*ManagedConnection, bool) {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	conn, exists := cp.connections[id]
	return conn, exists
}

// Put adds a connection to the pool.
func (cp *ConnectionPool) Put(id string, conn *ManagedConnection) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if len(cp.connections) >= cp.maxSize {
		return fmt.Errorf("connection pool is full (max size: %d)", cp.maxSize)
	}

	if _, exists := cp.connections[id]; exists {
		return fmt.Errorf("connection with ID %s already exists", id)
	}

	cp.connections[id] = conn
	cp.totalCreated.Add(1)
	return nil
}

// Remove removes a connection from the pool.
func (cp *ConnectionPool) Remove(id string) bool {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if conn, exists := cp.connections[id]; exists {
		delete(cp.connections, id)
		cp.totalDestroyed.Add(1)

		// Shutdown the connection
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeoutSec*time.Second)
			defer cancel()
			_ = conn.Shutdown(ctx)
		}()

		return true
	}

	return false
}

// Size returns the current number of connections.
func (cp *ConnectionPool) Size() int {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return len(cp.connections)
}

// List returns all connection IDs.
func (cp *ConnectionPool) List() []string {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	ids := make([]string, 0, len(cp.connections))
	for id := range cp.connections {
		ids = append(ids, id)
	}
	return ids
}

// Shutdown shuts down all connections in the pool.
func (cp *ConnectionPool) Shutdown(ctx context.Context) error {
	cp.mu.Lock()
	connections := make([]*ManagedConnection, 0, len(cp.connections))
	for _, conn := range cp.connections {
		connections = append(connections, conn)
	}
	cp.connections = make(map[string]*ManagedConnection)
	cp.mu.Unlock()

	// Shutdown all connections concurrently
	var wg sync.WaitGroup
	errCh := make(chan error, len(connections))

	for _, conn := range connections {
		wg.Add(1)
		go func(c *ManagedConnection) {
			defer wg.Done()
			if err := c.Shutdown(ctx); err != nil {
				errCh <- err
			}
		}(conn)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during pool shutdown: %v", errs)
	}

	return nil
}

// GetMetrics returns pool metrics.
func (cp *ConnectionPool) GetMetrics() PoolMetrics {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	return PoolMetrics{
		CurrentSize:    len(cp.connections),
		MaxSize:        cp.maxSize,
		TotalCreated:   cp.totalCreated.Load(),
		TotalDestroyed: cp.totalDestroyed.Load(),
	}
}

// PoolMetrics contains connection pool metrics.
type PoolMetrics struct {
	CurrentSize    int
	MaxSize        int
	TotalCreated   int64
	TotalDestroyed int64
}
