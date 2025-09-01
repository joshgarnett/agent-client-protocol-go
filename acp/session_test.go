package acp

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionStatus(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected string
	}{
		{SessionStatusActive, "active"},
		{SessionStatusPending, "pending"},
		{SessionStatusCancelled, "cancelled"},
		{SessionStatusCompleted, "completed"},
		{SessionStatusError, "error"},
		{SessionStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestSessionState(t *testing.T) {
	t.Run("NewSessionState", func(t *testing.T) {
		sessionID := api.SessionId("test-session-123")
		state := NewSessionState(sessionID)

		assert.Equal(t, sessionID, state.ID)
		assert.Equal(t, SessionStatusPending, state.Status)
		assert.NotZero(t, state.CreatedAt)
		assert.Equal(t, state.CreatedAt, state.LastActive)
		assert.NotNil(t, state.Metadata)
	})

	t.Run("UpdateActivity", func(t *testing.T) {
		state := NewSessionState("test-session")
		originalTime := state.LastActive

		time.Sleep(10 * time.Millisecond)
		state.UpdateActivity()

		assert.True(t, state.LastActive.After(originalTime))
	})

	t.Run("SetStatus", func(t *testing.T) {
		state := NewSessionState("test-session")
		assert.Equal(t, SessionStatusPending, state.GetStatus())

		state.SetStatus(SessionStatusActive)
		assert.Equal(t, SessionStatusActive, state.GetStatus())

		state.SetStatus(SessionStatusCompleted)
		assert.Equal(t, SessionStatusCompleted, state.GetStatus())
	})

	t.Run("Metadata Operations", func(t *testing.T) {
		state := NewSessionState("test-session")

		// Set metadata
		state.SetMetadata("key1", "value1")
		state.SetMetadata("key2", 42)
		state.SetMetadata("key3", true)

		// Get metadata
		val, exists := state.GetMetadata("key1")
		assert.True(t, exists)
		assert.Equal(t, "value1", val)

		val, exists = state.GetMetadata("key2")
		assert.True(t, exists)
		assert.Equal(t, 42, val)

		val, exists = state.GetMetadata("key3")
		assert.True(t, exists)
		assert.Equal(t, true, val)

		// Non-existent key
		val, exists = state.GetMetadata("nonexistent")
		assert.False(t, exists)
		assert.Nil(t, val)
	})

	t.Run("IsActive", func(t *testing.T) {
		state := NewSessionState("test-session")
		assert.False(t, state.IsActive())

		state.SetStatus(SessionStatusActive)
		assert.True(t, state.IsActive())

		state.SetStatus(SessionStatusCompleted)
		assert.False(t, state.IsActive())
	})

	t.Run("Duration", func(t *testing.T) {
		state := NewSessionState("test-session")

		time.Sleep(50 * time.Millisecond)
		duration := state.Duration()

		assert.GreaterOrEqual(t, duration, 50*time.Millisecond)
		assert.Less(t, duration, 200*time.Millisecond)
	})

	t.Run("Concurrent Access", func(t *testing.T) {
		state := NewSessionState("test-session")

		var wg sync.WaitGroup
		iterations := 100

		// Concurrent status updates
		wg.Add(iterations)
		for i := range iterations {
			go func(i int) {
				defer wg.Done()
				if i%2 == 0 {
					state.SetStatus(SessionStatusActive)
				} else {
					state.SetStatus(SessionStatusPending)
				}
			}(i)
		}

		// Concurrent metadata updates
		wg.Add(iterations)
		for i := range iterations {
			go func(i int) {
				defer wg.Done()
				state.SetMetadata(fmt.Sprintf("key%d", i), i)
			}(i)
		}

		// Concurrent reads
		wg.Add(iterations)
		for i := range iterations {
			go func(i int) {
				defer wg.Done()
				_ = state.GetStatus()
				state.GetMetadata(fmt.Sprintf("key%d", i/2))
			}(i)
		}

		wg.Wait()

		// Should have metadata from concurrent updates
		assert.NotEmpty(t, state.Metadata)
	})
}

func TestSessionManager(t *testing.T) {
	t.Run("CreateSession", func(t *testing.T) {
		manager := NewSessionManager()
		sessionID := api.SessionId("session-1")

		session, err := manager.CreateSession(sessionID)
		require.NoError(t, err)
		assert.Equal(t, sessionID, session.ID)
		assert.Equal(t, SessionStatusPending, session.GetStatus())

		// Try to create duplicate
		_, err = manager.CreateSession(sessionID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("GetSession", func(t *testing.T) {
		manager := NewSessionManager()
		sessionID := api.SessionId("session-1")

		// Create session
		created, err := manager.CreateSession(sessionID)
		require.NoError(t, err)

		// Get session
		retrieved, exists := manager.GetSession(sessionID)
		assert.True(t, exists)
		assert.Equal(t, created.ID, retrieved.ID)

		// Get non-existent session
		_, exists = manager.GetSession("nonexistent")
		assert.False(t, exists)
	})

	t.Run("UpdateSession", func(t *testing.T) {
		manager := NewSessionManager()
		sessionID := api.SessionId("session-1")

		// Create session
		_, err := manager.CreateSession(sessionID)
		require.NoError(t, err)

		// Update status
		err = manager.UpdateSession(sessionID, SessionStatusActive)
		require.NoError(t, err)

		session, exists := manager.GetSession(sessionID)
		assert.True(t, exists)
		assert.Equal(t, SessionStatusActive, session.GetStatus())

		// Update non-existent session
		err = manager.UpdateSession("nonexistent", SessionStatusActive)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("DeleteSession", func(t *testing.T) {
		manager := NewSessionManager()
		sessionID := api.SessionId("session-1")

		// Create and delete session
		_, err := manager.CreateSession(sessionID)
		require.NoError(t, err)

		err = manager.DeleteSession(sessionID)
		require.NoError(t, err)

		// Should no longer exist
		_, exists := manager.GetSession(sessionID)
		assert.False(t, exists)

		// Delete non-existent session
		err = manager.DeleteSession("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("ListSessions", func(t *testing.T) {
		manager := NewSessionManager()

		// Create multiple sessions
		for i := range 5 {
			_, err := manager.CreateSession(api.SessionId(fmt.Sprintf("session-%d", i)))
			require.NoError(t, err)
		}

		sessions := manager.ListSessions()
		assert.Len(t, sessions, 5)
	})

	t.Run("GetActiveSessions", func(t *testing.T) {
		manager := NewSessionManager()

		// Create sessions with different statuses
		for i := range 5 {
			sessionID := api.SessionId(fmt.Sprintf("session-%d", i))
			_, err := manager.CreateSession(sessionID)
			require.NoError(t, err)

			if i%2 == 0 {
				manager.UpdateSession(sessionID, SessionStatusActive)
			}
		}

		activeSessions := manager.GetActiveSessions()
		assert.Len(t, activeSessions, 3) // 0, 2, 4 are active

		for _, session := range activeSessions {
			assert.Equal(t, SessionStatusActive, session.GetStatus())
		}
	})

	t.Run("GetSessionsByStatus", func(t *testing.T) {
		manager := NewSessionManager()

		// Create sessions with various statuses
		statuses := []SessionStatus{
			SessionStatusPending,
			SessionStatusActive,
			SessionStatusActive,
			SessionStatusCompleted,
			SessionStatusCancelled,
		}

		for i, status := range statuses {
			sessionID := api.SessionId(fmt.Sprintf("session-%d", i))
			_, err := manager.CreateSession(sessionID)
			require.NoError(t, err)
			if status != SessionStatusPending {
				manager.UpdateSession(sessionID, status)
			}
		}

		// Check each status
		pending := manager.GetSessionsByStatus(SessionStatusPending)
		assert.Len(t, pending, 1)

		active := manager.GetSessionsByStatus(SessionStatusActive)
		assert.Len(t, active, 2)

		completed := manager.GetSessionsByStatus(SessionStatusCompleted)
		assert.Len(t, completed, 1)

		cancelled := manager.GetSessionsByStatus(SessionStatusCancelled)
		assert.Len(t, cancelled, 1)

		errorSessions := manager.GetSessionsByStatus(SessionStatusError)
		assert.Empty(t, errorSessions)
	})

	t.Run("Count Methods", func(t *testing.T) {
		manager := NewSessionManager()
		assert.Equal(t, 0, manager.Count())

		// Create sessions with different statuses
		for i := range 5 {
			sessionID := api.SessionId(fmt.Sprintf("session-%d", i))
			_, err := manager.CreateSession(sessionID)
			require.NoError(t, err)
		}
		assert.Equal(t, 5, manager.Count())

		// Set some to active
		manager.UpdateSession("session-0", SessionStatusActive)
		manager.UpdateSession("session-1", SessionStatusActive)
		manager.UpdateSession("session-2", SessionStatusCompleted)

		assert.Equal(t, 2, manager.CountByStatus(SessionStatusActive))
		assert.Equal(t, 1, manager.CountByStatus(SessionStatusCompleted))
		assert.Equal(t, 2, manager.CountByStatus(SessionStatusPending)) // 3 and 4
	})

	t.Run("Callbacks", func(t *testing.T) {
		manager := NewSessionManager()

		var createdSession *SessionState
		var updatedSession *SessionState
		var deletedSession *SessionState

		manager.OnSessionCreate(func(session *SessionState) {
			createdSession = session
		})

		manager.OnSessionUpdate(func(session *SessionState) {
			updatedSession = session
		})

		manager.OnSessionDelete(func(session *SessionState) {
			deletedSession = session
		})

		// Create session
		sessionID := api.SessionId("test-session")
		_, err := manager.CreateSession(sessionID)
		require.NoError(t, err)

		// Wait for async callback
		time.Sleep(10 * time.Millisecond)
		assert.NotNil(t, createdSession)
		assert.Equal(t, sessionID, createdSession.ID)

		// Update session
		err = manager.UpdateSession(sessionID, SessionStatusActive)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		assert.NotNil(t, updatedSession)
		assert.Equal(t, SessionStatusActive, updatedSession.GetStatus())

		// Delete session
		err = manager.DeleteSession(sessionID)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		assert.NotNil(t, deletedSession)
		assert.Equal(t, sessionID, deletedSession.ID)
	})

	t.Run("CleanupInactiveSessions", func(t *testing.T) {
		manager := NewSessionManager()

		// Create sessions
		for i := range 3 {
			sessionID := api.SessionId(fmt.Sprintf("session-%d", i))
			session, err := manager.CreateSession(sessionID)
			require.NoError(t, err)

			// Manually set LastActive for testing
			if i == 0 {
				// Make this one old
				session.LastActive = time.Now().Add(-2 * time.Hour)
			}
		}

		// Cleanup sessions inactive for more than 1 hour
		cleaned := manager.CleanupInactiveSessions(time.Hour)
		assert.Equal(t, 1, cleaned)
		assert.Equal(t, 2, manager.Count())

		// Session 0 should be gone
		_, exists := manager.GetSession("session-0")
		assert.False(t, exists)
	})
}

func TestMemorySessionStore(t *testing.T) {
	t.Run("Save and Load", func(t *testing.T) {
		store := NewMemorySessionStore()
		session := NewSessionState("test-session")
		session.SetMetadata("key", "value")

		// Save session
		err := store.Save(session)
		require.NoError(t, err)

		// Load session
		loaded, err := store.Load("test-session")
		require.NoError(t, err)
		assert.Equal(t, session.ID, loaded.ID)
		assert.Equal(t, session.Status, loaded.Status)

		// Check metadata was copied
		val, exists := loaded.GetMetadata("key")
		assert.True(t, exists)
		assert.Equal(t, "value", val)

		// Load non-existent session
		_, err = store.Load("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Save Nil Session", func(t *testing.T) {
		store := NewMemorySessionStore()
		err := store.Save(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Delete", func(t *testing.T) {
		store := NewMemorySessionStore()
		session := NewSessionState("test-session")

		// Save and delete
		err := store.Save(session)
		require.NoError(t, err)

		err = store.Delete("test-session")
		require.NoError(t, err)

		// Should no longer exist
		_, err = store.Load("test-session")
		require.Error(t, err)

		// Delete non-existent
		err = store.Delete("nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("List", func(t *testing.T) {
		store := NewMemorySessionStore()

		// Save multiple sessions
		for i := range 3 {
			session := NewSessionState(api.SessionId(fmt.Sprintf("session-%d", i)))
			err := store.Save(session)
			require.NoError(t, err)
		}

		// List all
		sessions, err := store.List()
		require.NoError(t, err)
		assert.Len(t, sessions, 3)
	})

	t.Run("Exists", func(t *testing.T) {
		store := NewMemorySessionStore()
		session := NewSessionState("test-session")

		// Check before save
		exists, err := store.Exists("test-session")
		require.NoError(t, err)
		assert.False(t, exists)

		// Save and check
		err = store.Save(session)
		require.NoError(t, err)

		exists, err = store.Exists("test-session")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("Isolation", func(t *testing.T) {
		store := NewMemorySessionStore()
		session := NewSessionState("test-session")
		session.SetMetadata("key", "original")

		// Save session
		err := store.Save(session)
		require.NoError(t, err)

		// Modify original
		session.SetMetadata("key", "modified")

		// Load should have original value
		loaded, err := store.Load("test-session")
		require.NoError(t, err)

		val, _ := loaded.GetMetadata("key")
		assert.Equal(t, "original", val)

		// Modify loaded
		loaded.SetMetadata("key", "loaded-modified")

		// Re-load should still have original
		reloaded, err := store.Load("test-session")
		require.NoError(t, err)

		val, _ = reloaded.GetMetadata("key")
		assert.Equal(t, "original", val)
	})
}

func TestStateTransitions(t *testing.T) {
	t.Run("Valid Transitions", func(t *testing.T) {
		tests := []struct {
			from  ConnectionState
			to    ConnectionState
			valid bool
		}{
			{StateUninitialized, StateInitialized, true},
			{StateInitialized, StateAuthenticated, true},
			{StateInitialized, StateSessionReady, true},
			{StateAuthenticated, StateSessionReady, true},
			{StateSessionReady, StateSessionReady, true}, // Can handle multiple sessions
			{StateUninitialized, StateAuthenticated, false},
			{StateUninitialized, StateSessionReady, false},
			{StateAuthenticated, StateInitialized, false},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("%v to %v", tt.from, tt.to), func(t *testing.T) {
				validStates, exists := stateTransitions[tt.from]
				if !exists && !tt.valid {
					// No transitions defined, correctly invalid
					return
				}

				found := false
				for _, state := range validStates {
					if state == tt.to {
						found = true
						break
					}
				}
				assert.Equal(t, tt.valid, found)
			})
		}
	})
}
