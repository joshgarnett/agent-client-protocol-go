package acp

import (
	"context"
	"errors"
	"fmt"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// ToolCallBuilder provides a fluent interface for building tool calls.
type ToolCallBuilder struct {
	toolCall api.ToolCall
}

// NewToolCall creates a new tool call builder with the specified ID and title.
func NewToolCall(id api.ToolCallId, title string) *ToolCallBuilder {
	return &ToolCallBuilder{
		toolCall: api.ToolCall{
			ToolCallId: id,
			Title:      title,
			Status:     api.ToolCallStatusPending,
		},
	}
}

// WithKind sets the tool kind.
func (tcb *ToolCallBuilder) WithKind(kind interface{}) *ToolCallBuilder {
	tcb.toolCall.Kind = kind
	return tcb
}

// WithContent sets the content blocks.
func (tcb *ToolCallBuilder) WithContent(content []api.ToolCallContentElem) *ToolCallBuilder {
	tcb.toolCall.Content = content
	return tcb
}

// AddContent appends a content block.
func (tcb *ToolCallBuilder) AddContent(content api.ToolCallContentElem) *ToolCallBuilder {
	tcb.toolCall.Content = append(tcb.toolCall.Content, content)
	return tcb
}

// WithLocation adds a location to the tool call.
func (tcb *ToolCallBuilder) WithLocation(location api.ToolCallLocation) *ToolCallBuilder {
	tcb.toolCall.Locations = append(tcb.toolCall.Locations, location)
	return tcb
}

// WithLocations sets the locations collection.
func (tcb *ToolCallBuilder) WithLocations(locations []api.ToolCallLocation) *ToolCallBuilder {
	tcb.toolCall.Locations = locations
	return tcb
}

// WithRawInput sets the raw input parameters.
func (tcb *ToolCallBuilder) WithRawInput(input interface{}) *ToolCallBuilder {
	tcb.toolCall.RawInput = input
	return tcb
}

// WithRawOutput sets the raw output.
func (tcb *ToolCallBuilder) WithRawOutput(output interface{}) *ToolCallBuilder {
	tcb.toolCall.RawOutput = output
	return tcb
}

// WithStatus sets the tool call status.
func (tcb *ToolCallBuilder) WithStatus(status api.ToolCallStatus) *ToolCallBuilder {
	tcb.toolCall.Status = status
	return tcb
}

// Build returns the constructed tool call.
func (tcb *ToolCallBuilder) Build() api.ToolCall {
	return tcb.toolCall
}

// ToUpdate creates a ToolCallUpdate from the current builder state.
func (tcb *ToolCallBuilder) ToUpdate() api.ToolCallUpdate {
	update := api.ToolCallUpdate{
		ToolCallId: tcb.toolCall.ToolCallId,
	}

	// Only include non-empty fields in the update
	if tcb.toolCall.Title != "" {
		title := tcb.toolCall.Title
		update.Title = &title
	}

	if tcb.toolCall.Kind != "" {
		update.Kind = tcb.toolCall.Kind
	}

	if len(tcb.toolCall.Content) > 0 {
		// Convert ToolCallContentElem to ToolCallUpdateContentElem
		updateContent := make([]api.ToolCallUpdateContentElem, len(tcb.toolCall.Content))
		for i, c := range tcb.toolCall.Content {
			// ToolCallContentElem and ToolCallUpdateContentElem should be compatible
			// This assumes the types are interface{} or compatible structures
			updateContent[i] = api.ToolCallUpdateContentElem(c)
		}
		update.Content = updateContent
	}

	if len(tcb.toolCall.Locations) > 0 {
		update.Locations = tcb.toolCall.Locations
	}

	if tcb.toolCall.RawInput != nil {
		update.RawInput = tcb.toolCall.RawInput
	}

	if tcb.toolCall.RawOutput != nil {
		update.RawOutput = tcb.toolCall.RawOutput
	}

	if tcb.toolCall.Status != "" {
		update.Status = tcb.toolCall.Status
	}

	return update
}

// ToolCallUpdateBuilder provides a fluent interface for building tool call updates.
type ToolCallUpdateBuilder struct {
	update api.ToolCallUpdate
}

// NewToolCallUpdate creates a new tool call update builder.
func NewToolCallUpdate(id api.ToolCallId) *ToolCallUpdateBuilder {
	return &ToolCallUpdateBuilder{
		update: api.ToolCallUpdate{
			ToolCallId: id,
		},
	}
}

// WithTitle sets the title in the update.
func (tub *ToolCallUpdateBuilder) WithTitle(title string) *ToolCallUpdateBuilder {
	tub.update.Title = &title
	return tub
}

// WithKind sets the tool kind in the update.
func (tub *ToolCallUpdateBuilder) WithKind(kind interface{}) *ToolCallUpdateBuilder {
	tub.update.Kind = kind
	return tub
}

// WithContent sets the content collection in the update.
func (tub *ToolCallUpdateBuilder) WithContent(content []api.ToolCallUpdateContentElem) *ToolCallUpdateBuilder {
	tub.update.Content = content
	return tub
}

// WithLocations sets the locations collection in the update.
func (tub *ToolCallUpdateBuilder) WithLocations(locations []api.ToolCallLocation) *ToolCallUpdateBuilder {
	tub.update.Locations = locations
	return tub
}

// WithRawInput sets the raw input in the update.
func (tub *ToolCallUpdateBuilder) WithRawInput(input interface{}) *ToolCallUpdateBuilder {
	tub.update.RawInput = input
	return tub
}

// WithRawOutput sets the raw output in the update.
func (tub *ToolCallUpdateBuilder) WithRawOutput(output interface{}) *ToolCallUpdateBuilder {
	tub.update.RawOutput = output
	return tub
}

// WithStatus sets the status in the update.
func (tub *ToolCallUpdateBuilder) WithStatus(status interface{}) *ToolCallUpdateBuilder {
	tub.update.Status = status
	return tub
}

// Build returns the constructed tool call update.
func (tub *ToolCallUpdateBuilder) Build() api.ToolCallUpdate {
	return tub.update
}

// Status transition helpers

// CanTransition checks if a tool call status transition is valid.
func CanTransition(from, to api.ToolCallStatus) bool {
	switch from {
	case api.ToolCallStatusPending:
		// Can transition to any state from pending
		return true
	case api.ToolCallStatusInProgress:
		// Can transition to completed or failed
		return to == api.ToolCallStatusCompleted || to == api.ToolCallStatusFailed
	case api.ToolCallStatusCompleted, api.ToolCallStatusFailed:
		// Terminal states - no transitions allowed
		return false
	default:
		return false
	}
}

// TransitionToolCall updates a tool call's status if the transition is valid.
func TransitionToolCall(toolCall *api.ToolCall, status api.ToolCallStatus) error {
	if toolCall == nil {
		return errors.New("tool call cannot be nil")
	}

	if !CanTransition(toolCall.Status, status) {
		return fmt.Errorf("invalid status transition from %v to %v", toolCall.Status, status)
	}

	toolCall.Status = status
	return nil
}

// Progress reporting

// ToolCallProgress represents progress information for a tool call.
type ToolCallProgress struct {
	Current int
	Total   int
	Message string
}

// ReportProgress sends a progress update for a tool call.
func ReportProgress(ctx context.Context, conn *AgentConnection, sessionID api.SessionId,
	toolCallID api.ToolCallId, progress ToolCallProgress) error {
	// Create a progress message as content
	progressText := fmt.Sprintf("Progress: %d/%d", progress.Current, progress.Total)
	if progress.Message != "" {
		progressText = fmt.Sprintf("%s - %s", progressText, progress.Message)
	}

	// Create an update with progress content
	update := NewToolCallUpdate(toolCallID).
		WithContent([]api.ToolCallUpdateContentElem{
			progressText, // Assuming string can be used as content element
		}).
		Build()

	return conn.SendToolCallUpdate(ctx, sessionID, update)
}

// Integration helpers for AgentConnection

// SendToolCallUpdate sends a tool call update through the agent connection.
func (a *AgentConnection) SendToolCallUpdate(ctx context.Context, sessionID api.SessionId,
	update api.ToolCallUpdate) error {
	// Create a session update with the tool call update
	sessionUpdate := api.NewSessionUpdateToolCallUpdate(
		update.Content,
		update.Kind,
		update.Locations,
		update.RawInput,
		update.RawOutput,
		update.Status,
		update.Title,
		&update.ToolCallId,
	)

	// Send the session notification
	return a.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    sessionUpdate,
	})
}

// SendNewToolCall sends a new tool call through the agent connection.
func (a *AgentConnection) SendNewToolCall(ctx context.Context, sessionID api.SessionId,
	toolCall api.ToolCall) error {
	// Convert content to []interface{}
	var content []interface{}
	for _, c := range toolCall.Content {
		content = append(content, c)
	}

	// Convert locations to []interface{}
	var locations []interface{}
	for _, l := range toolCall.Locations {
		locations = append(locations, l)
	}

	// Convert Kind to *ToolKind if it's a string
	var kind *api.ToolKind
	if toolCall.Kind != nil {
		if kindStr, ok := toolCall.Kind.(string); ok && kindStr != "" {
			toolKind := api.ToolKind(kindStr)
			kind = &toolKind
		}
	}

	// Create a session update with the new tool call
	sessionUpdate := api.NewSessionUpdateToolCall(
		content,
		kind,
		locations,
		toolCall.RawInput,
		toolCall.RawOutput,
		&toolCall.Status,
		toolCall.Title,
		&toolCall.ToolCallId,
	)

	// Send the session notification
	return a.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    sessionUpdate,
	})
}

// Tool call management utilities

// FindToolCallByID searches for a tool call by ID in a collection.
func FindToolCallByID(toolCalls []api.ToolCall, id api.ToolCallId) *api.ToolCall {
	for i := range toolCalls {
		if toolCalls[i].ToolCallId == id {
			return &toolCalls[i]
		}
	}
	return nil
}

// GetToolCallsByStatus filters tool calls by status.
func GetToolCallsByStatus(toolCalls []api.ToolCall, status api.ToolCallStatus) []api.ToolCall {
	var result []api.ToolCall
	for _, tc := range toolCalls {
		if tc.Status == status {
			result = append(result, tc)
		}
	}
	return result
}

// GetToolCallsByKind filters tool calls by kind.
func GetToolCallsByKind(toolCalls []api.ToolCall, kind interface{}) []api.ToolCall {
	var result []api.ToolCall
	for _, tc := range toolCalls {
		// Compare kinds - both are interface{}
		if fmt.Sprintf("%v", tc.Kind) == fmt.Sprintf("%v", kind) {
			result = append(result, tc)
		}
	}
	return result
}

// ValidateToolCall performs basic validation on a tool call.
func ValidateToolCall(toolCall *api.ToolCall) error {
	if toolCall == nil {
		return errors.New("tool call cannot be nil")
	}

	if toolCall.ToolCallId == "" {
		return errors.New("tool call ID cannot be empty")
	}

	if toolCall.Title == "" {
		return errors.New("tool call title cannot be empty")
	}

	// Validate status
	switch toolCall.Status {
	case api.ToolCallStatusPending, api.ToolCallStatusInProgress,
		api.ToolCallStatusCompleted, api.ToolCallStatusFailed:
		// Valid status
	default:
		return fmt.Errorf("invalid tool call status: %v", toolCall.Status)
	}

	// Validate kind if specified
	if toolCall.Kind != nil {
		if kindStr, ok := toolCall.Kind.(string); ok {
			switch api.ToolKind(kindStr) {
			case api.ToolKindThink, api.ToolKindEdit, api.ToolKindDelete,
				api.ToolKindExecute, api.ToolKindFetch, api.ToolKindMove,
				api.ToolKindRead, api.ToolKindSearch, api.ToolKindOther:
				// Valid kind
			default:
				return fmt.Errorf("invalid tool call kind: %v", kindStr)
			}
		}
	}

	return nil
}

// CreateLocation creates a new tool call location.
func CreateLocation(path string, line int) api.ToolCallLocation {
	return api.ToolCallLocation{
		Path: path,
		Line: &line,
	}
}

// CreateLocationSimple creates a new tool call location without line number.
func CreateLocationSimple(path string) api.ToolCallLocation {
	return api.ToolCallLocation{
		Path: path,
	}
}
