package acp

import (
	"fmt"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/joshgarnett/agent-client-protocol-go/util"
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
	LastActive *util.AtomicValue[time.Time]
	Status     *util.AtomicValue[SessionStatus]
	Metadata   *util.SyncMap[string, interface{}]
}

// NewSessionState creates a new session state.
func NewSessionState(id api.SessionId) *SessionState {
	now := time.Now()
	ss := &SessionState{
		ID:         id,
		CreatedAt:  now,
		LastActive: util.NewAtomicValue(now),
		Status:     util.NewAtomicValue(SessionStatusPending),
		Metadata:   util.NewSyncMap[string, interface{}](),
	}
	return ss
}

// UpdateActivity updates the last active timestamp.
func (ss *SessionState) UpdateActivity() {
	ss.LastActive.Store(time.Now())
}

// SetStatus updates the session status.
func (ss *SessionState) SetStatus(status SessionStatus) {
	ss.Status.Store(status)
	ss.LastActive.Store(time.Now())
}

// GetStatus returns the current session status.
func (ss *SessionState) GetStatus() SessionStatus {
	return ss.Status.Load()
}

// SetMetadata sets a metadata key-value pair.
func (ss *SessionState) SetMetadata(key string, value interface{}) {
	ss.Metadata.Store(key, value)
}

// GetMetadata retrieves a metadata value by key.
func (ss *SessionState) GetMetadata(key string) (interface{}, bool) {
	return ss.Metadata.Load(key)
}

// IsActive returns true if the session is currently active.
func (ss *SessionState) IsActive() bool {
	return ss.Status.Load() == SessionStatusActive
}

// Duration returns how long the session has been running.
func (ss *SessionState) Duration() time.Duration {
	return time.Since(ss.CreatedAt)
}

// sessionCallbacks holds all session callback functions.
type sessionCallbacks struct {
	onCreate func(*SessionState)
	onUpdate func(*SessionState)
	onDelete func(*SessionState)
}

// SessionManager manages multiple sessions.
type SessionManager struct {
	sessions  *util.SyncMap[api.SessionId, *SessionState]
	callbacks *util.AtomicValue[*sessionCallbacks]
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:  util.NewSyncMap[api.SessionId, *SessionState](),
		callbacks: util.NewAtomicValue(&sessionCallbacks{}),
	}
}

// CreateSession creates a new session with the given ID.
func (sm *SessionManager) CreateSession(id api.SessionId) (*SessionState, error) {
	session := NewSessionState(id)

	// Try to add the session, will return the existing one if already present
	actual, loaded := sm.sessions.LoadOrStore(id, session)
	if loaded {
		return nil, fmt.Errorf("session %v already exists", id)
	}

	callbacks := sm.callbacks.Load()
	if callbacks.onCreate != nil {
		go callbacks.onCreate(actual)
	}

	return session, nil
}

// GetSession retrieves a session by ID.
func (sm *SessionManager) GetSession(id api.SessionId) (*SessionState, bool) {
	return sm.sessions.Load(id)
}

// UpdateSession updates the status of a session.
func (sm *SessionManager) UpdateSession(id api.SessionId, status SessionStatus) error {
	session, exists := sm.sessions.Load(id)
	if !exists {
		return fmt.Errorf("session %v not found", id)
	}

	session.SetStatus(status)

	callbacks := sm.callbacks.Load()
	if callbacks.onUpdate != nil {
		go callbacks.onUpdate(session)
	}

	return nil
}

// DeleteSession removes a session from the manager.
func (sm *SessionManager) DeleteSession(id api.SessionId) error {
	session, exists := sm.sessions.LoadAndDelete(id)
	if !exists {
		return fmt.Errorf("session %v not found", id)
	}

	callbacks := sm.callbacks.Load()
	if callbacks.onDelete != nil {
		go callbacks.onDelete(session)
	}

	return nil
}

// ListSessions returns all sessions.
func (sm *SessionManager) ListSessions() []*SessionState {
	sessionMap := sm.sessions.GetAll()
	sessions := make([]*SessionState, 0, len(sessionMap))
	for _, session := range sessionMap {
		sessions = append(sessions, session)
	}
	return sessions
}

// GetActiveSessions returns all active sessions.
func (sm *SessionManager) GetActiveSessions() []*SessionState {
	sessionMap := sm.sessions.GetAll()
	var active []*SessionState
	for _, session := range sessionMap {
		if session.IsActive() {
			active = append(active, session)
		}
	}
	return active
}

// GetSessionsByStatus returns all sessions with the specified status.
func (sm *SessionManager) GetSessionsByStatus(status SessionStatus) []*SessionState {
	sessionMap := sm.sessions.GetAll()
	var result []*SessionState
	for _, session := range sessionMap {
		if session.GetStatus() == status {
			result = append(result, session)
		}
	}
	return result
}

// Count returns the total number of sessions.
func (sm *SessionManager) Count() int {
	return sm.sessions.Count()
}

// CountByStatus returns the count of sessions by status.
func (sm *SessionManager) CountByStatus(status SessionStatus) int {
	sessionMap := sm.sessions.GetAll()
	count := 0
	for _, session := range sessionMap {
		if session.GetStatus() == status {
			count++
		}
	}
	return count
}

// OnSessionCreate sets the callback for session creation.
func (sm *SessionManager) OnSessionCreate(callback func(*SessionState)) {
	sm.callbacks.Update(func(old *sessionCallbacks) *sessionCallbacks {
		return &sessionCallbacks{
			onCreate: callback,
			onUpdate: old.onUpdate,
			onDelete: old.onDelete,
		}
	})
}

// OnSessionUpdate sets the callback for session updates.
func (sm *SessionManager) OnSessionUpdate(callback func(*SessionState)) {
	sm.callbacks.Update(func(old *sessionCallbacks) *sessionCallbacks {
		return &sessionCallbacks{
			onCreate: old.onCreate,
			onUpdate: callback,
			onDelete: old.onDelete,
		}
	})
}

// OnSessionDelete sets the callback for session deletion.
func (sm *SessionManager) OnSessionDelete(callback func(*SessionState)) {
	sm.callbacks.Update(func(old *sessionCallbacks) *sessionCallbacks {
		return &sessionCallbacks{
			onCreate: old.onCreate,
			onUpdate: old.onUpdate,
			onDelete: callback,
		}
	})
}

// CleanupInactiveSessions removes sessions that have been inactive for the specified duration.
func (sm *SessionManager) CleanupInactiveSessions(inactiveDuration time.Duration) int {
	now := time.Now()
	cleaned := 0

	// Get all sessions and identify inactive ones
	sessionMap := sm.sessions.GetAll()
	var toDelete []api.SessionId
	var deletedSessions []*SessionState

	for id, session := range sessionMap {
		if now.Sub(session.LastActive.Load()) > inactiveDuration {
			toDelete = append(toDelete, id)
			deletedSessions = append(deletedSessions, session)
		}
	}

	// Delete inactive sessions
	for _, id := range toDelete {
		sm.sessions.Delete(id)
		cleaned++
	}

	// Call callbacks for deleted sessions
	callbacks := sm.callbacks.Load()
	if callbacks.onDelete != nil {
		for _, session := range deletedSessions {
			go callbacks.onDelete(session)
		}
	}

	return cleaned
}
