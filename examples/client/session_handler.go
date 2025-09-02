package main

import (
	"context"
	"fmt"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// ============================================================================
// Session Update Display Constants
// ============================================================================

const (
	StatusPending   = "PENDING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

// ============================================================================
// Session Update Processing
// ============================================================================

// handleSessionUpdate processes session updates from agent.
func handleSessionUpdate(_ context.Context, params *api.SessionNotification) error {
	update, ok := params.Update.(*api.SessionUpdate)
	if !ok {
		fmt.Printf("[UPDATE] Untyped session update: %+v\n", params.Update)
		return nil
	}

	handleTypedSessionUpdate(update)
	return nil
}

// handleTypedSessionUpdate formats and displays session updates.
func handleTypedSessionUpdate(update *api.SessionUpdate) {
	switch update.Type {
	case api.SessionUpdateTypeAgentMessageChunk:
		handleAgentMessageChunk(update.GetAgentMessageChunk())
	case api.SessionUpdateTypeAgentThoughtChunk:
		handleAgentThoughtChunk(update.GetAgentThoughtChunk())
	case api.SessionUpdateTypePlan:
		handlePlanUpdate(update.GetPlan())
	case api.SessionUpdateTypeToolCall:
		handleToolCallUpdate(update.GetToolCall())
	case api.SessionUpdateTypeToolCallUpdate:
		handleToolCallUpdateNotification(update.GetToolCallUpdate())
	case api.SessionUpdateTypeUserMessageChunk:
		handleUserMessageChunk(update.GetUserMessageChunk())
	default:
		fmt.Printf("[UPDATE] Unknown update type: %s\n", update.Type)
	}
}

// handleAgentMessageChunk displays agent message chunks.
func handleAgentMessageChunk(chunk *api.SessionUpdateAgentMessageChunk) {
	if chunk == nil {
		return
	}
	printContentBlock(*chunk.Content)
}

// printContentBlock displays content blocks by type.
func printContentBlock(content api.ContentBlock) {
	if textContent := content.GetText(); textContent != nil {
		fmt.Print(textContent.Text)
		return
	}
	if imageContent := content.GetImage(); imageContent != nil {
		fmt.Print("[Image]")
		return
	}
	if audioContent := content.GetAudio(); audioContent != nil {
		fmt.Print("[Audio]")
		return
	}
	if resourceContent := content.GetResource(); resourceContent != nil {
		fmt.Print("[Resource]")
		return
	}
	if linkContent := content.GetResourceLink(); linkContent != nil {
		fmt.Printf("[Link: %s]", linkContent.Uri)
		return
	}
	fmt.Print("[Unknown content]")
}

// handleAgentThoughtChunk processes agent thoughts.
func handleAgentThoughtChunk(chunk *api.SessionUpdateAgentThoughtChunk) {
	if chunk == nil {
		return
	}

	// Log internal agent reasoning
	if textContent := chunk.Content.GetText(); textContent != nil {
		fmt.Printf("[THOUGHT] %s\n", textContent.Text)
	}
}

// handlePlanUpdate displays agent plans.
func handlePlanUpdate(plan *api.SessionUpdatePlan) {
	if plan == nil {
		return
	}

	fmt.Printf("\n[PLAN] Agent created a plan with %d steps:\n", len(plan.Entries))
	for i, entryInterface := range plan.Entries {
		status, title := parsePlanEntry(entryInterface)
		fmt.Printf("  %s %d. %s\n", status, i+1, title)
	}
	fmt.Println()
}

// parsePlanEntry extracts plan entry details.
func parsePlanEntry(entryInterface interface{}) (string, string) {
	status := StatusPending
	title := "Unknown task"

	entryMap, mapOk := entryInterface.(map[string]interface{})
	if !mapOk {
		return status, title
	}

	// Extract status
	if statusStr, exists := entryMap["status"]; exists {
		if statusVal, statusOk := statusStr.(string); statusOk {
			status = mapStatusString(statusVal)
		}
	}

	// Extract title
	if titleStr, titleExists := entryMap["title"]; titleExists {
		if titleVal, titleOk := titleStr.(string); titleOk {
			title = titleVal
		}
	} else if contentStr, contentExists := entryMap["content"]; contentExists {
		if contentVal, contentOk := contentStr.(string); contentOk {
			title = contentVal
		}
	}

	return status, title
}

// mapStatusString converts status values to display format.
func mapStatusString(statusVal string) string {
	switch statusVal {
	case "completed":
		return StatusCompleted
	case "failed":
		return StatusFailed
	default:
		return StatusPending
	}
}

// handleToolCallUpdate displays tool call status.
func handleToolCallUpdate(toolCall *api.SessionUpdateToolCall) {
	if toolCall == nil {
		return
	}

	status := StatusPending
	switch {
	case toolCall.Status != nil && *toolCall.Status == api.ToolCallStatusCompleted:
		status = StatusCompleted
	case toolCall.Status != nil && *toolCall.Status == api.ToolCallStatusFailed:
		status = StatusFailed
	case toolCall.Status != nil && *toolCall.Status == api.ToolCallStatusPending:
		status = StatusPending
	}

	kindStr := "unknown"
	if toolCall.Kind != nil {
		kindStr = string(*toolCall.Kind)
	}

	fmt.Printf("\n[TOOL] %s %s (%s)\n", status, toolCall.Title, kindStr)

	if len(toolCall.Locations) > 0 {
		fmt.Println("   Locations:")
		for _, locInterface := range toolCall.Locations {
			if locMap, ok := locInterface.(map[string]interface{}); ok {
				if path, exists := locMap["path"]; exists {
					fmt.Printf("     - %v\n", path)
				}
			} else {
				fmt.Printf("     - %v\n", locInterface)
			}
		}
	}

	// Show any content returned by the tool
	if len(toolCall.Content) > 0 {
		fmt.Println("   Content:")
		for _, content := range toolCall.Content {
			if contentItem, ok := content.(api.ContentBlock); ok {
				if textContent := contentItem.GetText(); textContent != nil {
					fmt.Printf("     %s\n", textContent.Text)
				}
			}
		}
	}

	fmt.Println()
}

// handleToolCallUpdateNotification displays tool call updates.
func handleToolCallUpdateNotification(toolCallUpdate *api.SessionUpdateToolCallUpdate) {
	if toolCallUpdate == nil {
		return
	}

	status := extractToolCallUpdateStatus(toolCallUpdate.Status)
	fmt.Printf("[TOOL UPDATE] %s %s\n", status, toolCallUpdate.Title)
	printToolCallUpdateContent(toolCallUpdate.Content)
}

// extractToolCallUpdateStatus gets tool call status.
func extractToolCallUpdateStatus(statusInterface interface{}) string {
	if statusInterface == nil {
		return "UPDATING"
	}

	statusStr, ok := statusInterface.(string)
	if !ok {
		return "UPDATING"
	}

	return mapStatusString(statusStr)
}

// printToolCallUpdateContent displays tool call results.
func printToolCallUpdateContent(contentInterface interface{}) {
	if contentInterface == nil {
		return
	}

	contentSlice, sliceOk := contentInterface.([]interface{})
	if !sliceOk || len(contentSlice) == 0 {
		return
	}

	fmt.Println("   Updated content:")
	for _, contentItem := range contentSlice {
		printToolCallContentItem(contentItem)
	}
}

// printToolCallContentItem displays a content item.
func printToolCallContentItem(contentItem interface{}) {
	contentMap, mapOk := contentItem.(map[string]interface{})
	if !mapOk {
		fmt.Printf("     %v\n", contentItem)
		return
	}

	contentType, typeExists := contentMap["type"]
	if !typeExists || contentType != "text" {
		fmt.Printf("     %v\n", contentItem)
		return
	}

	text, textExists := contentMap["text"]
	if textExists {
		fmt.Printf("     %v\n", text)
	}
}

// handleUserMessageChunk processes echoed user messages.
func handleUserMessageChunk(chunk *api.SessionUpdateUserMessageChunk) {
	if chunk == nil {
		return
	}

	// Log echoed messages for debugging
	if textContent := chunk.Content.GetText(); textContent != nil {
		fmt.Printf("[USER ECHO] %s\n", textContent.Text)
	}
}
