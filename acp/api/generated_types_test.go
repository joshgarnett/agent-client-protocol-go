package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test enum types.
func TestToolKind(t *testing.T) {
	tests := []struct {
		name    string
		value   ToolKind
		valid   bool
		jsonStr string
	}{
		{"Read", ToolKindRead, true, `"read"`},
		{"Edit", ToolKindEdit, true, `"edit"`},
		{"Execute", ToolKindExecute, true, `"execute"`},
		{"Invalid", ToolKind("invalid"), false, `"invalid"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation
			assert.Equal(t, tt.valid, tt.value.IsValid())

			// Test string representation
			assert.Equal(t, string(tt.value), tt.value.String())

			// Test JSON marshaling for valid values
			if tt.valid {
				data, err := json.Marshal(tt.value)
				require.NoError(t, err)
				assert.Equal(t, tt.jsonStr, string(data))

				// Test JSON unmarshaling
				var unmarshaled ToolKind
				err = json.Unmarshal(data, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, tt.value, unmarshaled)
			} else {
				// Test that invalid values fail marshaling
				_, err := json.Marshal(tt.value)
				assert.Error(t, err)
			}
		})
	}

	// Test AllToolKindValues
	allValues := AllToolKindValues()
	assert.Len(t, allValues, 9) // Update this count based on actual enum values
	for _, value := range allValues {
		assert.True(t, value.IsValid())
	}
}

func TestToolCallStatus(t *testing.T) {
	tests := []struct {
		name    string
		value   ToolCallStatus
		valid   bool
		jsonStr string
	}{
		{"Pending", ToolCallStatusPending, true, `"pending"`},
		{"InProgress", ToolCallStatusInProgress, true, `"in_progress"`},
		{"Completed", ToolCallStatusCompleted, true, `"completed"`},
		{"Failed", ToolCallStatusFailed, true, `"failed"`},
		{"Invalid", ToolCallStatus("invalid"), false, `"invalid"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.value.IsValid())
			assert.Equal(t, string(tt.value), tt.value.String())

			if tt.valid {
				data, err := json.Marshal(tt.value)
				require.NoError(t, err)
				assert.Equal(t, tt.jsonStr, string(data))

				var unmarshaled ToolCallStatus
				err = json.Unmarshal(data, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, tt.value, unmarshaled)
			} else {
				_, err := json.Marshal(tt.value)
				assert.Error(t, err)
			}
		})
	}
}

func TestStopReason(t *testing.T) {
	tests := []struct {
		name    string
		value   StopReason
		valid   bool
		jsonStr string
	}{
		{"EndTurn", StopReasonEndTurn, true, `"end_turn"`},
		{"MaxTokens", StopReasonMaxTokens, true, `"max_tokens"`},
		{"Refusal", StopReasonRefusal, true, `"refusal"`},
		{"Invalid", StopReason("invalid"), false, `"invalid"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.value.IsValid())
			assert.Equal(t, string(tt.value), tt.value.String())

			if tt.valid {
				data, err := json.Marshal(tt.value)
				require.NoError(t, err)
				assert.Equal(t, tt.jsonStr, string(data))

				var unmarshaled StopReason
				err = json.Unmarshal(data, &unmarshaled)
				require.NoError(t, err)
				assert.Equal(t, tt.value, unmarshaled)
			} else {
				_, err := json.Marshal(tt.value)
				assert.Error(t, err)
			}
		})
	}
}

// Test ContentBlock union type.
func TestContentBlock(t *testing.T) {
	t.Run("TextContent", func(t *testing.T) {
		// Test constructor
		block := NewContentBlockText(nil, "Hello, world!")

		assert.Equal(t, ContentBlockTypeText, block.Type)
		assert.True(t, block.IsContentBlock(ContentBlockTypeText))
		assert.False(t, block.IsContentBlock(ContentBlockTypeImage))

		// Test getter
		textContent := block.GetText()
		require.NotNil(t, textContent)
		assert.Equal(t, "Hello, world!", textContent.Text)

		// Test other getters return nil
		assert.Nil(t, block.GetImage())
		assert.Nil(t, block.GetAudio())

		// Test JSON marshaling
		data, err := json.Marshal(block)
		require.NoError(t, err)

		var unmarshaled ContentBlock
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ContentBlockTypeText, unmarshaled.Type)
		textContent = unmarshaled.GetText()
		require.NotNil(t, textContent)
		assert.Equal(t, "Hello, world!", textContent.Text)
	})

	t.Run("ImageContent", func(t *testing.T) {
		// Test constructor
		block := NewContentBlockImage(nil, "base64data", "image/png", "https://example.com/image.png")

		assert.Equal(t, ContentBlockTypeImage, block.Type)
		assert.True(t, block.IsContentBlock(ContentBlockTypeImage))

		// Test getter
		imageContent := block.GetImage()
		require.NotNil(t, imageContent)
		assert.Equal(t, "base64data", imageContent.Data)
		assert.Equal(t, "image/png", imageContent.Mimetype)
		assert.Equal(t, "https://example.com/image.png", imageContent.Uri)

		// Test JSON marshaling
		data, err := json.Marshal(block)
		require.NoError(t, err)

		var unmarshaled ContentBlock
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ContentBlockTypeImage, unmarshaled.Type)
		imageContent = unmarshaled.GetImage()
		require.NotNil(t, imageContent)
		assert.Equal(t, "base64data", imageContent.Data)
		assert.Equal(t, "image/png", imageContent.Mimetype)
	})

	t.Run("InvalidType", func(t *testing.T) {
		// Test unmarshaling with invalid type
		invalidJSON := `{"type": "invalid_type", "text": "test"}`

		var block ContentBlock
		err := json.Unmarshal([]byte(invalidJSON), &block)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ContentBlock type")
	})

	t.Run("MissingVariant", func(t *testing.T) {
		// Test marshaling with missing variant data
		block := ContentBlock{Type: ContentBlockTypeText}

		_, err := json.Marshal(block)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Text field is required")
	})
}

// Test SessionUpdate union type.
func TestSessionUpdate(t *testing.T) {
	t.Run("AgentMessageChunk", func(t *testing.T) {
		textBlock := NewContentBlockText(nil, "Agent response")
		update := NewSessionUpdateAgentMessageChunk(textBlock)

		assert.Equal(t, SessionUpdateTypeAgentMessageChunk, update.Type)
		assert.True(t, update.IsSessionUpdate(SessionUpdateTypeAgentMessageChunk))

		// Test getter
		chunk := update.GetAgentMessageChunk()
		require.NotNil(t, chunk)
		assert.Equal(t, textBlock, chunk.Content)

		// Test other getters return nil
		assert.Nil(t, update.GetPlan())
		assert.Nil(t, update.GetToolCall())

		// Test JSON marshaling
		data, err := json.Marshal(update)
		require.NoError(t, err)

		var unmarshaled SessionUpdate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, SessionUpdateTypeAgentMessageChunk, unmarshaled.Type)
		chunk = unmarshaled.GetAgentMessageChunk()
		require.NotNil(t, chunk)
	})

	t.Run("Plan", func(t *testing.T) {
		entries := []interface{}{
			map[string]interface{}{"task": "Step 1"},
			map[string]interface{}{"task": "Step 2"},
		}
		update := NewSessionUpdatePlan(entries)

		assert.Equal(t, SessionUpdateTypePlan, update.Type)

		// Test getter
		plan := update.GetPlan()
		require.NotNil(t, plan)
		assert.Len(t, plan.Entries, 2)

		// Test JSON marshaling
		data, err := json.Marshal(update)
		require.NoError(t, err)

		var unmarshaled SessionUpdate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, SessionUpdateTypePlan, unmarshaled.Type)
		plan = unmarshaled.GetPlan()
		require.NotNil(t, plan)
		assert.Len(t, plan.Entries, 2)
	})

	t.Run("ToolCall", func(t *testing.T) {
		content := []interface{}{"output1", "output2"}
		kind := ToolKindExecute
		status := ToolCallStatusCompleted

		update := NewSessionUpdateToolCall(
			content,
			&kind,
			nil, // locations
			"input data",
			"output data",
			&status,
			"Test Tool Call",
			nil, // toolCallId
		)

		assert.Equal(t, SessionUpdateTypeToolCall, update.Type)

		// Test getter
		toolCall := update.GetToolCall()
		require.NotNil(t, toolCall)
		assert.Equal(t, content, toolCall.Content)
		assert.Equal(t, &kind, toolCall.Kind)
		assert.Equal(t, &status, toolCall.Status)
		assert.Equal(t, "Test Tool Call", toolCall.Title)

		// Test JSON marshaling
		data, err := json.Marshal(update)
		require.NoError(t, err)

		var unmarshaled SessionUpdate
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, SessionUpdateTypeToolCall, unmarshaled.Type)
		toolCall = unmarshaled.GetToolCall()
		require.NotNil(t, toolCall)
		assert.Equal(t, "Test Tool Call", toolCall.Title)
	})
}

// Test ToolCallContent union type.
func TestToolCallContent(t *testing.T) {
	t.Run("Content", func(t *testing.T) {
		contentBlock := NewContentBlockText(nil, "Tool output")
		toolContent := NewToolCallContentContent(contentBlock)

		assert.Equal(t, ToolCallContentTypeContent, toolContent.Type)
		assert.True(t, toolContent.IsToolCallContent(ToolCallContentTypeContent))

		// Test getter
		content := toolContent.GetContent()
		require.NotNil(t, content)
		assert.Equal(t, contentBlock, content.Content)

		// Test other getters return nil
		assert.Nil(t, toolContent.GetDiff())
		assert.Nil(t, toolContent.GetTerminal())

		// Test JSON marshaling
		data, err := json.Marshal(toolContent)
		require.NoError(t, err)

		var unmarshaled ToolCallContent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ToolCallContentTypeContent, unmarshaled.Type)
		content = unmarshaled.GetContent()
		require.NotNil(t, content)
	})

	t.Run("Diff", func(t *testing.T) {
		toolContent := NewToolCallContentDiff(
			"new file content",
			"old file content",
			"/path/to/file.txt",
		)

		assert.Equal(t, ToolCallContentTypeDiff, toolContent.Type)

		// Test getter
		diff := toolContent.GetDiff()
		require.NotNil(t, diff)
		assert.Equal(t, "new file content", diff.Newtext)
		assert.Equal(t, "old file content", diff.Oldtext)
		assert.Equal(t, "/path/to/file.txt", diff.Path)

		// Test JSON marshaling
		data, err := json.Marshal(toolContent)
		require.NoError(t, err)

		var unmarshaled ToolCallContent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ToolCallContentTypeDiff, unmarshaled.Type)
		diff = unmarshaled.GetDiff()
		require.NotNil(t, diff)
		assert.Equal(t, "new file content", diff.Newtext)
		assert.Equal(t, "/path/to/file.txt", diff.Path)
	})

	t.Run("Terminal", func(t *testing.T) {
		toolContent := NewToolCallContentTerminal("terminal-123")

		assert.Equal(t, ToolCallContentTypeTerminal, toolContent.Type)

		// Test getter
		terminal := toolContent.GetTerminal()
		require.NotNil(t, terminal)
		assert.Equal(t, "terminal-123", terminal.Terminalid)

		// Test JSON marshaling
		data, err := json.Marshal(toolContent)
		require.NoError(t, err)

		var unmarshaled ToolCallContent
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, ToolCallContentTypeTerminal, unmarshaled.Type)
		terminal = unmarshaled.GetTerminal()
		require.NotNil(t, terminal)
		assert.Equal(t, "terminal-123", terminal.Terminalid)
	})
}

// Test plan entry enums.
func TestPlanEntryStatus(t *testing.T) {
	tests := []struct {
		name  string
		value PlanEntryStatus
		valid bool
	}{
		{"Pending", PlanEntryStatusPending, true},
		{"InProgress", PlanEntryStatusInProgress, true},
		{"Completed", PlanEntryStatusCompleted, true},
		{"Invalid", PlanEntryStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.value.IsValid())
		})
	}
}

func TestPlanEntryPriority(t *testing.T) {
	tests := []struct {
		name  string
		value PlanEntryPriority
		valid bool
	}{
		{"Low", PlanEntryPriorityLow, true},
		{"Medium", PlanEntryPriorityMedium, true},
		{"High", PlanEntryPriorityHigh, true},
		{"Invalid", PlanEntryPriority("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.value.IsValid())
		})
	}
}

func TestPermissionOptionKind(t *testing.T) {
	tests := []struct {
		name  string
		value PermissionOptionKind
		valid bool
	}{
		{"AllowAlways", PermissionOptionKindAllowAlways, true},
		{"AllowOnce", PermissionOptionKindAllowOnce, true},
		{"RejectAlways", PermissionOptionKindRejectAlways, true},
		{"RejectOnce", PermissionOptionKindRejectOnce, true},
		{"Invalid", PermissionOptionKind("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.value.IsValid())
		})
	}
}
