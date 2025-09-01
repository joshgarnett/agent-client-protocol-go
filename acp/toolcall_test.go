package acp

import (
	"fmt"
	"testing"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolCallBuilder(t *testing.T) {
	t.Run("NewToolCall", func(t *testing.T) {
		id := api.ToolCallId("tool-123")
		title := "Test Tool Call"

		builder := NewToolCall(id, title)
		toolCall := builder.Build()

		assert.Equal(t, id, toolCall.ToolCallId)
		assert.Equal(t, title, toolCall.Title)
		assert.Equal(t, api.ToolCallStatusPending, toolCall.Status)
	})

	t.Run("Builder Methods", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")

		// Chain builder methods
		builder.
			WithKind(api.ToolKindRead).
			WithRawInput(map[string]string{"file": "test.txt"}).
			WithRawOutput(map[string]string{"content": "file content"}).
			WithStatus(api.ToolCallStatusInProgress)

		toolCall := builder.Build()

		assert.Equal(t, api.ToolKindRead, toolCall.Kind)
		assert.NotNil(t, toolCall.RawInput)
		assert.NotNil(t, toolCall.RawOutput)
		assert.Equal(t, api.ToolCallStatusInProgress, toolCall.Status)
	})

	t.Run("WithContent", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")

		content := []api.ToolCallContentElem{
			"Line 1",
			"Line 2",
			map[string]interface{}{"type": "info", "message": "test"},
		}

		builder.WithContent(content)
		toolCall := builder.Build()

		assert.Len(t, toolCall.Content, 3)
	})

	t.Run("AddContent", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")

		builder.
			AddContent("First content").
			AddContent("Second content").
			AddContent(map[string]string{"key": "value"})

		toolCall := builder.Build()
		assert.Len(t, toolCall.Content, 3)
	})

	t.Run("WithLocation", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")

		location := api.ToolCallLocation{
			Path: "/path/to/file.go",
			Line: IntPtr(42),
		}

		builder.WithLocation(location)
		toolCall := builder.Build()

		require.Len(t, toolCall.Locations, 1)
		assert.Equal(t, "/path/to/file.go", toolCall.Locations[0].Path)
		assert.Equal(t, 42, *toolCall.Locations[0].Line)
	})

	t.Run("WithLocations", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")

		locations := []api.ToolCallLocation{
			{Path: "/file1.go", Line: IntPtr(10)},
			{Path: "/file2.go", Line: IntPtr(20)},
			{Path: "/file3.go"},
		}

		builder.WithLocations(locations)
		toolCall := builder.Build()

		assert.Len(t, toolCall.Locations, 3)
	})

	t.Run("ToUpdate", func(t *testing.T) {
		builder := NewToolCall("tool-123", "Test Tool")
		builder.
			WithKind(api.ToolKindExecute).
			WithStatus(api.ToolCallStatusCompleted).
			WithRawOutput(map[string]string{"result": "success"})

		update := builder.ToUpdate()

		assert.Equal(t, api.ToolCallId("tool-123"), update.ToolCallId)
		assert.NotNil(t, update.Title)
		assert.Equal(t, "Test Tool", *update.Title)
		assert.Equal(t, api.ToolKindExecute, update.Kind)
		assert.Equal(t, api.ToolCallStatusCompleted, update.Status)
		assert.NotNil(t, update.RawOutput)
	})
}

func TestToolCallUpdateBuilder(t *testing.T) {
	t.Run("NewToolCallUpdate", func(t *testing.T) {
		id := api.ToolCallId("tool-123")
		builder := NewToolCallUpdate(id)
		update := builder.Build()

		assert.Equal(t, id, update.ToolCallId)
	})

	t.Run("Update Builder Methods", func(t *testing.T) {
		builder := NewToolCallUpdate("tool-123")

		builder.
			WithTitle("Updated Title").
			WithKind(api.ToolKindSearch).
			WithStatus(api.ToolCallStatusInProgress).
			WithRawInput(map[string]string{"query": "test"}).
			WithRawOutput(map[string]interface{}{"results": []string{"result1", "result2"}})

		update := builder.Build()

		assert.NotNil(t, update.Title)
		assert.Equal(t, "Updated Title", *update.Title)
		assert.Equal(t, api.ToolKindSearch, update.Kind)
		assert.Equal(t, api.ToolCallStatusInProgress, update.Status)
		assert.NotNil(t, update.RawInput)
		assert.NotNil(t, update.RawOutput)
	})

	t.Run("Update with Content", func(t *testing.T) {
		builder := NewToolCallUpdate("tool-123")

		content := []api.ToolCallUpdateContentElem{
			"Update 1",
			"Update 2",
			map[string]interface{}{"progress": 50},
		}

		builder.WithContent(content)
		update := builder.Build()

		assert.Len(t, update.Content, 3)
	})

	t.Run("Update with Locations", func(t *testing.T) {
		builder := NewToolCallUpdate("tool-123")

		locations := []api.ToolCallLocation{
			{Path: "/updated/file1.go"},
			{Path: "/updated/file2.go", Line: IntPtr(100)},
		}

		builder.WithLocations(locations)
		update := builder.Build()

		assert.Len(t, update.Locations, 2)
	})
}

func TestStatusTransitions(t *testing.T) {
	tests := []struct {
		from  api.ToolCallStatus
		to    api.ToolCallStatus
		valid bool
	}{
		// From Pending
		{api.ToolCallStatusPending, api.ToolCallStatusInProgress, true},
		{api.ToolCallStatusPending, api.ToolCallStatusCompleted, true},
		{api.ToolCallStatusPending, api.ToolCallStatusFailed, true},

		// From InProgress
		{api.ToolCallStatusInProgress, api.ToolCallStatusCompleted, true},
		{api.ToolCallStatusInProgress, api.ToolCallStatusFailed, true},
		{api.ToolCallStatusInProgress, api.ToolCallStatusPending, false},

		// From Completed (terminal state)
		{api.ToolCallStatusCompleted, api.ToolCallStatusPending, false},
		{api.ToolCallStatusCompleted, api.ToolCallStatusInProgress, false},
		{api.ToolCallStatusCompleted, api.ToolCallStatusFailed, false},

		// From Failed (terminal state)
		{api.ToolCallStatusFailed, api.ToolCallStatusPending, false},
		{api.ToolCallStatusFailed, api.ToolCallStatusInProgress, false},
		{api.ToolCallStatusFailed, api.ToolCallStatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v to %v", tt.from, tt.to), func(t *testing.T) {
			result := CanTransition(tt.from, tt.to)
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestTransitionToolCall(t *testing.T) {
	t.Run("Valid Transition", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Status:     api.ToolCallStatusPending,
		}

		err := TransitionToolCall(toolCall, api.ToolCallStatusInProgress)
		require.NoError(t, err)
		assert.Equal(t, api.ToolCallStatusInProgress, toolCall.Status)

		err = TransitionToolCall(toolCall, api.ToolCallStatusCompleted)
		require.NoError(t, err)
		assert.Equal(t, api.ToolCallStatusCompleted, toolCall.Status)
	})

	t.Run("Invalid Transition", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Status:     api.ToolCallStatusCompleted,
		}

		err := TransitionToolCall(toolCall, api.ToolCallStatusInProgress)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")
		// Status should remain unchanged
		assert.Equal(t, api.ToolCallStatusCompleted, toolCall.Status)
	})

	t.Run("Nil ToolCall", func(t *testing.T) {
		err := TransitionToolCall(nil, api.ToolCallStatusInProgress)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

func TestToolCallManagement(t *testing.T) {
	t.Run("FindToolCallByID", func(t *testing.T) {
		toolCalls := []api.ToolCall{
			{ToolCallId: "tool-1", Title: "Tool 1"},
			{ToolCallId: "tool-2", Title: "Tool 2"},
			{ToolCallId: "tool-3", Title: "Tool 3"},
		}

		found := FindToolCallByID(toolCalls, "tool-2")
		require.NotNil(t, found)
		assert.Equal(t, "Tool 2", found.Title)

		notFound := FindToolCallByID(toolCalls, "tool-99")
		assert.Nil(t, notFound)

		// Empty slice
		notFound = FindToolCallByID([]api.ToolCall{}, "tool-1")
		assert.Nil(t, notFound)
	})

	t.Run("GetToolCallsByStatus", func(t *testing.T) {
		toolCalls := []api.ToolCall{
			{ToolCallId: "tool-1", Status: api.ToolCallStatusPending},
			{ToolCallId: "tool-2", Status: api.ToolCallStatusInProgress},
			{ToolCallId: "tool-3", Status: api.ToolCallStatusCompleted},
			{ToolCallId: "tool-4", Status: api.ToolCallStatusPending},
			{ToolCallId: "tool-5", Status: api.ToolCallStatusFailed},
		}

		pending := GetToolCallsByStatus(toolCalls, api.ToolCallStatusPending)
		assert.Len(t, pending, 2)

		inProgress := GetToolCallsByStatus(toolCalls, api.ToolCallStatusInProgress)
		assert.Len(t, inProgress, 1)

		completed := GetToolCallsByStatus(toolCalls, api.ToolCallStatusCompleted)
		assert.Len(t, completed, 1)

		failed := GetToolCallsByStatus(toolCalls, api.ToolCallStatusFailed)
		assert.Len(t, failed, 1)
	})

	t.Run("GetToolCallsByKind", func(t *testing.T) {
		toolCalls := []api.ToolCall{
			{ToolCallId: "tool-1", Kind: api.ToolKindRead},
			{ToolCallId: "tool-2", Kind: api.ToolKindExecute},
			{ToolCallId: "tool-3", Kind: api.ToolKindRead},
			{ToolCallId: "tool-4", Kind: api.ToolKindSearch},
		}

		readCalls := GetToolCallsByKind(toolCalls, api.ToolKindRead)
		assert.Len(t, readCalls, 2)

		executeCalls := GetToolCallsByKind(toolCalls, api.ToolKindExecute)
		assert.Len(t, executeCalls, 1)

		// Test with interface{} comparison
		searchCalls := GetToolCallsByKind(toolCalls, api.ToolKindSearch)
		assert.Len(t, searchCalls, 1)

		// No matches
		thinkCalls := GetToolCallsByKind(toolCalls, api.ToolKindThink)
		assert.Empty(t, thinkCalls)
	})
}

func TestValidateToolCall(t *testing.T) {
	t.Run("Valid ToolCall", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Title:      "Valid Tool",
			Status:     api.ToolCallStatusPending,
			Kind:       api.ToolKindRead,
		}

		err := ValidateToolCall(toolCall)
		assert.NoError(t, err)
	})

	t.Run("Nil ToolCall", func(t *testing.T) {
		err := ValidateToolCall(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Empty ID", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "",
			Title:      "Test Tool",
			Status:     api.ToolCallStatusPending,
		}

		err := ValidateToolCall(toolCall)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ID cannot be empty")
	})

	t.Run("Empty Title", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Title:      "",
			Status:     api.ToolCallStatusPending,
		}

		err := ValidateToolCall(toolCall)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "title cannot be empty")
	})

	t.Run("Invalid Status", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Title:      "Test Tool",
			Status:     api.ToolCallStatus("invalid"),
		}

		err := ValidateToolCall(toolCall)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool call status")
	})

	t.Run("Invalid Kind", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Title:      "Test Tool",
			Status:     api.ToolCallStatusPending,
			Kind:       "invalid-kind",
		}

		err := ValidateToolCall(toolCall)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid tool call kind")
	})

	t.Run("Valid Without Kind", func(t *testing.T) {
		toolCall := &api.ToolCall{
			ToolCallId: "tool-123",
			Title:      "Test Tool",
			Status:     api.ToolCallStatusPending,
			// Kind is optional
		}

		err := ValidateToolCall(toolCall)
		assert.NoError(t, err)
	})
}

func TestCreateLocation(t *testing.T) {
	t.Run("CreateLocation with Line", func(t *testing.T) {
		location := CreateLocation("/path/to/file.go", 42)

		assert.Equal(t, "/path/to/file.go", location.Path)
		require.NotNil(t, location.Line)
		assert.Equal(t, 42, *location.Line)
	})

	t.Run("CreateLocationSimple", func(t *testing.T) {
		location := CreateLocationSimple("/path/to/file.go")

		assert.Equal(t, "/path/to/file.go", location.Path)
		assert.Nil(t, location.Line)
	})
}

func TestToolCallProgress(t *testing.T) {
	t.Run("ToolCallProgress", func(t *testing.T) {
		progress := ToolCallProgress{
			Current: 50,
			Total:   100,
			Message: "Processing files",
		}

		assert.Equal(t, 50, progress.Current)
		assert.Equal(t, 100, progress.Total)
		assert.Equal(t, "Processing files", progress.Message)
	})
}
