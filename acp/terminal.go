package acp

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// TerminalHandle provides a high-level abstraction for terminal management with lifecycle handling.
//
// This follows the pattern from the TypeScript reference implementation, providing
// a handle-based approach to terminal management with automatic resource cleanup.
type TerminalHandle struct {
	ID        string
	sessionID api.SessionId
	conn      *ClientConnection
	mu        sync.RWMutex
	released  bool
}

// NewTerminalHandle creates a new terminal handle.
func NewTerminalHandle(id string, sessionID api.SessionId, conn *ClientConnection) *TerminalHandle {
	return &TerminalHandle{
		ID:        id,
		sessionID: sessionID,
		conn:      conn,
	}
}

// CurrentOutput gets the current output from the terminal.
//
// Returns an error if the terminal handle has been released.
func (th *TerminalHandle) CurrentOutput(ctx context.Context) error {
	th.mu.RLock()
	released := th.released
	th.mu.RUnlock()

	if released {
		return errors.New("terminal handle has been released")
	}

	return th.conn.TerminalOutput(ctx, &api.TerminalOutputRequest{
		SessionId:  th.sessionID,
		TerminalId: th.ID,
	})
}

// WaitForExit waits for the terminal to exit and returns the exit information.
//
// Returns an error if the terminal handle has been released.
func (th *TerminalHandle) WaitForExit(ctx context.Context) (*api.WaitForTerminalExitResponse, error) {
	th.mu.RLock()
	released := th.released
	th.mu.RUnlock()

	if released {
		return nil, errors.New("terminal handle has been released")
	}

	return th.conn.TerminalWaitForExit(ctx, &api.WaitForTerminalExitRequest{
		SessionId:  th.sessionID,
		TerminalId: th.ID,
	})
}

// Release releases the terminal resources.
//
// This method is idempotent - calling it multiple times is safe.
func (th *TerminalHandle) Release(ctx context.Context) error {
	th.mu.Lock()
	defer th.mu.Unlock()

	if th.released {
		return nil // Already released
	}

	err := th.conn.TerminalRelease(ctx, &api.ReleaseTerminalRequest{
		SessionId:  th.sessionID,
		TerminalId: th.ID,
	})

	th.released = true
	return err
}

// Close provides the standard Go Close pattern for resource cleanup.
//
// This is equivalent to calling Release with a background context.
func (th *TerminalHandle) Close() error {
	return th.Release(context.Background())
}

// IsReleased returns true if the terminal handle has been released.
func (th *TerminalHandle) IsReleased() bool {
	th.mu.RLock()
	defer th.mu.RUnlock()
	return th.released
}

// TerminalManager manages the lifecycle of multiple terminals within a session.
//
// This provides centralized management and cleanup of terminal handles,
// ensuring proper resource cleanup when sessions end.
type TerminalManager struct {
	terminals map[string]*TerminalHandle
	mu        sync.RWMutex
	sessionID api.SessionId
	conn      *ClientConnection
}

// NewTerminalManager creates a new terminal manager for the given session.
func NewTerminalManager(sessionID api.SessionId, conn *ClientConnection) *TerminalManager {
	return &TerminalManager{
		terminals: make(map[string]*TerminalHandle),
		sessionID: sessionID,
		conn:      conn,
	}
}

// CreateTerminal creates a new terminal and returns a handle for it.
func (tm *TerminalManager) CreateTerminal(ctx context.Context,
	params *api.CreateTerminalRequest) (*TerminalHandle, error) {
	// Ensure the session ID matches
	if params.SessionId != tm.sessionID {
		return nil, fmt.Errorf("session ID mismatch: expected %v, got %v", tm.sessionID, params.SessionId)
	}

	response, err := tm.conn.TerminalCreate(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create terminal: %w", err)
	}

	handle := NewTerminalHandle(response.TerminalId, tm.sessionID, tm.conn)

	tm.mu.Lock()
	tm.terminals[response.TerminalId] = handle
	tm.mu.Unlock()

	return handle, nil
}

// GetTerminal retrieves a terminal handle by ID.
func (tm *TerminalManager) GetTerminal(id string) (*TerminalHandle, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	handle, exists := tm.terminals[id]
	return handle, exists
}

// ListTerminals returns all terminal handles.
func (tm *TerminalManager) ListTerminals() []*TerminalHandle {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	handles := make([]*TerminalHandle, 0, len(tm.terminals))
	for _, handle := range tm.terminals {
		handles = append(handles, handle)
	}
	return handles
}

// ReleaseTerminal releases a specific terminal by ID.
func (tm *TerminalManager) ReleaseTerminal(ctx context.Context, id string) error {
	tm.mu.Lock()
	handle, exists := tm.terminals[id]
	if exists {
		delete(tm.terminals, id)
	}
	tm.mu.Unlock()

	if !exists {
		return fmt.Errorf("terminal with ID %q not found", id)
	}

	return handle.Release(ctx)
}

// ReleaseAll releases all terminals managed by this manager.
//
// This is typically called when a session ends to ensure proper cleanup.
func (tm *TerminalManager) ReleaseAll(ctx context.Context) error {
	tm.mu.Lock()
	terminals := make([]*TerminalHandle, 0, len(tm.terminals))
	for _, handle := range tm.terminals {
		terminals = append(terminals, handle)
	}
	tm.terminals = make(map[string]*TerminalHandle) // Clear all
	tm.mu.Unlock()

	var errors []error
	for _, handle := range terminals {
		if err := handle.Release(ctx); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to release %d terminals: %w", len(errors), errors[0])
	}
	return nil
}

// Count returns the number of active terminals.
func (tm *TerminalManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.terminals)
}

// Enhanced ClientConnection methods

// CreateTerminalWithHandle creates a new terminal and returns a handle for it.
//
// This is an enhanced version of the basic TerminalCreate method that returns
// a handle for easier lifecycle management.
func (c *ClientConnection) CreateTerminalWithHandle(ctx context.Context,
	params *api.CreateTerminalRequest) (*TerminalHandle, error) {
	response, err := c.TerminalCreate(ctx, params)
	if err != nil {
		return nil, err
	}

	return NewTerminalHandle(response.TerminalId, params.SessionId, c), nil
}

// Terminal workflow helpers

// TerminalWorkflow provides a high-level interface for common terminal operations.
type TerminalWorkflow struct {
	handle *TerminalHandle
}

// NewTerminalWorkflow creates a new terminal workflow wrapper.
func NewTerminalWorkflow(handle *TerminalHandle) *TerminalWorkflow {
	return &TerminalWorkflow{handle: handle}
}

// Execute runs a command in the terminal and waits for completion.
//
// This is a convenience method that combines output monitoring and exit waiting.
func (tw *TerminalWorkflow) Execute(ctx context.Context, _ string) (*api.WaitForTerminalExitResponse, error) {
	if tw.handle.IsReleased() {
		return nil, errors.New("terminal handle has been released")
	}

	// Send command to terminal (this would typically be done through a separate mechanism)
	// For now, we just wait for the terminal to complete
	return tw.handle.WaitForExit(ctx)
}

// GetOutputAndRelease gets the current output and then releases the terminal.
//
// This is a convenience method for one-shot terminal operations.
func (tw *TerminalWorkflow) GetOutputAndRelease(ctx context.Context) error {
	defer tw.handle.Close()

	return tw.handle.CurrentOutput(ctx)
}

// Session integration

// SessionTerminalManager integrates terminal management with session lifecycle.
type SessionTerminalManager struct {
	managers map[api.SessionId]*TerminalManager
	mu       sync.RWMutex
	conn     *ClientConnection
}

// NewSessionTerminalManager creates a new session-level terminal manager.
func NewSessionTerminalManager(conn *ClientConnection) *SessionTerminalManager {
	return &SessionTerminalManager{
		managers: make(map[api.SessionId]*TerminalManager),
		conn:     conn,
	}
}

// GetManager returns the terminal manager for a specific session.
func (stm *SessionTerminalManager) GetManager(sessionID api.SessionId) *TerminalManager {
	stm.mu.Lock()
	defer stm.mu.Unlock()

	manager, exists := stm.managers[sessionID]
	if !exists {
		manager = NewTerminalManager(sessionID, stm.conn)
		stm.managers[sessionID] = manager
	}
	return manager
}

// ReleaseSession releases all terminals for a specific session.
func (stm *SessionTerminalManager) ReleaseSession(ctx context.Context, sessionID api.SessionId) error {
	stm.mu.Lock()
	manager, exists := stm.managers[sessionID]
	if exists {
		delete(stm.managers, sessionID)
	}
	stm.mu.Unlock()

	if exists {
		return manager.ReleaseAll(ctx)
	}
	return nil
}

// ReleaseAll releases all terminals across all sessions.
func (stm *SessionTerminalManager) ReleaseAll(ctx context.Context) error {
	stm.mu.Lock()
	managers := make([]*TerminalManager, 0, len(stm.managers))
	for _, manager := range stm.managers {
		managers = append(managers, manager)
	}
	stm.managers = make(map[api.SessionId]*TerminalManager)
	stm.mu.Unlock()

	var errors []error
	for _, manager := range managers {
		if err := manager.ReleaseAll(ctx); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to release terminals from %d sessions", len(errors))
	}
	return nil
}

// ActiveSessions returns the session IDs that have active terminals.
func (stm *SessionTerminalManager) ActiveSessions() []api.SessionId {
	stm.mu.RLock()
	defer stm.mu.RUnlock()

	sessions := make([]api.SessionId, 0, len(stm.managers))
	for sessionID := range stm.managers {
		sessions = append(sessions, sessionID)
	}
	return sessions
}
