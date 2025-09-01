package acp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// SessionStatus represents the current status of a session.
type SessionStatus int

const (
	// SessionStatusActive indicates the session is currently active.
	SessionStatusActive SessionStatus = iota
	// SessionStatusPending indicates the session is pending activation.
	SessionStatusPending
	// SessionStatusCancelled indicates the session was cancelled.
	SessionStatusCancelled
	// SessionStatusCompleted indicates the session completed successfully.
	SessionStatusCompleted
	// SessionStatusError indicates the session ended with an error.
	SessionStatusError
)

// String returns the string representation of the session status.
func (s SessionStatus) String() string {
	switch s {
	case SessionStatusActive:
		return "active"
	case SessionStatusPending:
		return "pending"
	case SessionStatusCancelled:
		return "cancelled"
	case SessionStatusCompleted:
		return "completed"
	case SessionStatusError:
		return "error"
	default:
		return "unknown"
	}
}

// SessionState represents the state of a single session.
type SessionState struct {
	ID         api.SessionId
	CreatedAt  time.Time
	LastActive time.Time
	Status     SessionStatus
	Metadata   map[string]interface{}
	mu         sync.RWMutex
}

// NewSessionState creates a new session state.
func NewSessionState(id api.SessionId) *SessionState {
	now := time.Now()
	return &SessionState{
		ID:         id,
		CreatedAt:  now,
		LastActive: now,
		Status:     SessionStatusPending,
		Metadata:   make(map[string]interface{}),
	}
}

// UpdateActivity updates the last active timestamp.
func (ss *SessionState) UpdateActivity() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.LastActive = time.Now()
}

// SetStatus updates the session status.
func (ss *SessionState) SetStatus(status SessionStatus) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Status = status
	ss.LastActive = time.Now()
}

// GetStatus returns the current session status.
func (ss *SessionState) GetStatus() SessionStatus {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.Status
}

// SetMetadata sets a metadata key-value pair.
func (ss *SessionState) SetMetadata(key string, value interface{}) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.Metadata[key] = value
}

// GetMetadata retrieves a metadata value by key.
func (ss *SessionState) GetMetadata(key string) (interface{}, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	value, exists := ss.Metadata[key]
	return value, exists
}

// IsActive returns true if the session is currently active.
func (ss *SessionState) IsActive() bool {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.Status == SessionStatusActive
}

// Duration returns how long the session has been running.
func (ss *SessionState) Duration() time.Duration {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return time.Since(ss.CreatedAt)
}

// SessionManager manages multiple sessions.
type SessionManager struct {
	sessions map[api.SessionId]*SessionState
	mu       sync.RWMutex

	// Callbacks
	onSessionCreate func(*SessionState)
	onSessionUpdate func(*SessionState)
	onSessionDelete func(*SessionState)
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[api.SessionId]*SessionState),
	}
}

// CreateSession creates a new session with the given ID.
func (sm *SessionManager) CreateSession(id api.SessionId) (*SessionState, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.sessions[id]; exists {
		return nil, fmt.Errorf("session %v already exists", id)
	}

	session := NewSessionState(id)
	sm.sessions[id] = session

	if sm.onSessionCreate != nil {
		go sm.onSessionCreate(session)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (sm *SessionManager) GetSession(id api.SessionId) (*SessionState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[id]
	return session, exists
}

// UpdateSession updates the status of a session.
func (sm *SessionManager) UpdateSession(id api.SessionId, status SessionStatus) error {
	sm.mu.RLock()
	session, exists := sm.sessions[id]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %v not found", id)
	}

	session.SetStatus(status)

	if sm.onSessionUpdate != nil {
		go sm.onSessionUpdate(session)
	}

	return nil
}

// DeleteSession removes a session from the manager.
func (sm *SessionManager) DeleteSession(id api.SessionId) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[id]
	if !exists {
		return fmt.Errorf("session %v not found", id)
	}

	delete(sm.sessions, id)

	if sm.onSessionDelete != nil {
		go sm.onSessionDelete(session)
	}

	return nil
}

// ListSessions returns all sessions.
func (sm *SessionManager) ListSessions() []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sessions := make([]*SessionState, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetActiveSessions returns all active sessions.
func (sm *SessionManager) GetActiveSessions() []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var active []*SessionState
	for _, session := range sm.sessions {
		if session.IsActive() {
			active = append(active, session)
		}
	}
	return active
}

// GetSessionsByStatus returns all sessions with the specified status.
func (sm *SessionManager) GetSessionsByStatus(status SessionStatus) []*SessionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var result []*SessionState
	for _, session := range sm.sessions {
		if session.GetStatus() == status {
			result = append(result, session)
		}
	}
	return result
}

// Count returns the total number of sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// CountByStatus returns the count of sessions by status.
func (sm *SessionManager) CountByStatus(status SessionStatus) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0
	for _, session := range sm.sessions {
		if session.GetStatus() == status {
			count++
		}
	}
	return count
}

// OnSessionCreate sets the callback for session creation.
func (sm *SessionManager) OnSessionCreate(callback func(*SessionState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onSessionCreate = callback
}

// OnSessionUpdate sets the callback for session updates.
func (sm *SessionManager) OnSessionUpdate(callback func(*SessionState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onSessionUpdate = callback
}

// OnSessionDelete sets the callback for session deletion.
func (sm *SessionManager) OnSessionDelete(callback func(*SessionState)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onSessionDelete = callback
}

// CleanupInactiveSessions removes sessions that have been inactive for the specified duration.
func (sm *SessionManager) CleanupInactiveSessions(inactiveDuration time.Duration) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for id, session := range sm.sessions {
		if now.Sub(session.LastActive) > inactiveDuration {
			delete(sm.sessions, id)
			cleaned++

			if sm.onSessionDelete != nil {
				go sm.onSessionDelete(session)
			}
		}
	}

	return cleaned
}

// Session persistence interfaces

// SessionStore defines the interface for session persistence.
type SessionStore interface {
	// Save persists a session state.
	Save(session *SessionState) error
	// Load retrieves a session state by ID.
	Load(id api.SessionId) (*SessionState, error)
	// Delete removes a session from storage.
	Delete(id api.SessionId) error
	// List returns all stored sessions.
	List() ([]*SessionState, error)
	// Exists checks if a session exists in storage.
	Exists(id api.SessionId) (bool, error)
}

// MemorySessionStore provides in-memory session storage.
type MemorySessionStore struct {
	sessions map[api.SessionId]*SessionState
	mu       sync.RWMutex
}

// NewMemorySessionStore creates a new in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[api.SessionId]*SessionState),
	}
}

// Save stores a session in memory.
func (m *MemorySessionStore) Save(session *SessionState) error {
	if session == nil {
		return errors.New("session cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy to avoid external mutations
	sessionCopy := &SessionState{
		ID:         session.ID,
		CreatedAt:  session.CreatedAt,
		LastActive: session.LastActive,
		Status:     session.Status,
		Metadata:   make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range session.Metadata {
		sessionCopy.Metadata[k] = v
	}

	m.sessions[session.ID] = sessionCopy
	return nil
}

// Load retrieves a session from memory.
func (m *MemorySessionStore) Load(id api.SessionId) (*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	if !exists {
		return nil, fmt.Errorf("session %v not found", id)
	}

	// Return a copy to avoid external mutations
	sessionCopy := &SessionState{
		ID:         session.ID,
		CreatedAt:  session.CreatedAt,
		LastActive: session.LastActive,
		Status:     session.Status,
		Metadata:   make(map[string]interface{}),
	}

	// Copy metadata
	for k, v := range session.Metadata {
		sessionCopy.Metadata[k] = v
	}

	return sessionCopy, nil
}

// Delete removes a session from memory.
func (m *MemorySessionStore) Delete(id api.SessionId) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.sessions[id]; !exists {
		return fmt.Errorf("session %v not found", id)
	}

	delete(m.sessions, id)
	return nil
}

// List returns all sessions from memory.
func (m *MemorySessionStore) List() ([]*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*SessionState, 0, len(m.sessions))
	for _, session := range m.sessions {
		// Return copies to avoid external mutations
		sessionCopy := &SessionState{
			ID:         session.ID,
			CreatedAt:  session.CreatedAt,
			LastActive: session.LastActive,
			Status:     session.Status,
			Metadata:   make(map[string]interface{}),
		}

		// Copy metadata
		for k, v := range session.Metadata {
			sessionCopy.Metadata[k] = v
		}

		sessions = append(sessions, sessionCopy)
	}

	return sessions, nil
}

// Exists checks if a session exists in memory.
func (m *MemorySessionStore) Exists(id api.SessionId) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.sessions[id]
	return exists, nil
}

// Enhanced connection state management

// StateChangeCallback is called when connection state changes.
type StateChangeCallback func(from, to ConnectionState)

// stateTransitions defines valid state transitions.
var stateTransitions = map[ConnectionState][]ConnectionState{
	StateUninitialized: {StateInitialized},
	StateInitialized:   {StateAuthenticated, StateSessionReady},
	StateAuthenticated: {StateSessionReady},
	StateSessionReady:  {StateSessionReady}, // Can handle multiple sessions
}

// CanTransitionTo checks if a state transition is valid.
func (a *AgentConnection) CanTransitionTo(newState ConnectionState) bool {
	a.mu.RLock()
	currentState := a.state
	a.mu.RUnlock()

	validStates, exists := stateTransitions[currentState]
	if !exists {
		return false
	}

	for _, state := range validStates {
		if state == newState {
			return true
		}
	}
	return false
}

// TransitionTo attempts to transition to a new state.
func (a *AgentConnection) TransitionTo(newState ConnectionState) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.CanTransitionTo(newState) {
		return fmt.Errorf("invalid state transition from %v to %v", a.state, newState)
	}

	oldState := a.state
	a.state = newState

	// Notify callbacks
	if a.stateChangeCallbacks != nil {
		for _, callback := range a.stateChangeCallbacks {
			go callback(oldState, newState)
		}
	}

	return nil
}

// OnStateChange registers a callback for state changes.
func (a *AgentConnection) OnStateChange(callback StateChangeCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stateChangeCallbacks == nil {
		a.stateChangeCallbacks = make([]StateChangeCallback, 0)
	}
	a.stateChangeCallbacks = append(a.stateChangeCallbacks, callback)
}

// Add similar methods for ClientConnection

// CanTransitionTo checks if a state transition is valid.
func (c *ClientConnection) CanTransitionTo(newState ConnectionState) bool {
	c.mu.RLock()
	currentState := c.state
	c.mu.RUnlock()

	validStates, exists := stateTransitions[currentState]
	if !exists {
		return false
	}

	for _, state := range validStates {
		if state == newState {
			return true
		}
	}
	return false
}

// TransitionTo attempts to transition to a new state.
func (c *ClientConnection) TransitionTo(newState ConnectionState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.CanTransitionTo(newState) {
		return fmt.Errorf("invalid state transition from %v to %v", c.state, newState)
	}

	oldState := c.state
	c.state = newState

	// Notify callbacks
	if c.stateChangeCallbacks != nil {
		for _, callback := range c.stateChangeCallbacks {
			go callback(oldState, newState)
		}
	}

	return nil
}

// OnStateChange registers a callback for state changes.
func (c *ClientConnection) OnStateChange(callback StateChangeCallback) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stateChangeCallbacks == nil {
		c.stateChangeCallbacks = make([]StateChangeCallback, 0)
	}
	c.stateChangeCallbacks = append(c.stateChangeCallbacks, callback)
}
