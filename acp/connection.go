package acp

import (
	"sync"
)

// ConnectionStateTracker tracks connection state with callback support.
type ConnectionStateTracker struct {
	state                ConnectionState
	stateChangeCallbacks []StateChangeCallback
	mu                   sync.RWMutex
}

// NewConnectionStateTracker creates a new connection state tracker.
func NewConnectionStateTracker() *ConnectionStateTracker {
	return &ConnectionStateTracker{
		state: StateUninitialized,
	}
}

// GetState returns the current state.
func (c *ConnectionStateTracker) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetState sets the connection state.
func (c *ConnectionStateTracker) SetState(state ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

// GetStateAndCallbacks returns the current state and callbacks.
func (c *ConnectionStateTracker) GetStateAndCallbacks() (ConnectionState, []StateChangeCallback) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state, c.stateChangeCallbacks
}

// OnStateChange registers a callback for state changes.
func (c *ConnectionStateTracker) OnStateChange(callback StateChangeCallback) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stateChangeCallbacks == nil {
		c.stateChangeCallbacks = make([]StateChangeCallback, 0)
	}
	c.stateChangeCallbacks = append(c.stateChangeCallbacks, callback)
}
