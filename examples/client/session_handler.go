package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// handleSessionUpdate handles session update notifications from the agent.
func handleSessionUpdate(_ context.Context, params *api.SessionNotification) error {
	fmt.Fprintf(os.Stderr, "Received session update for session: %v\n", params.SessionId)

	// Use type-safe generated types to handle different session update types
	update, ok := params.Update.(*api.SessionUpdate)
	if !ok {
		// Fallback for non-typed updates (legacy support)
		fmt.Fprintf(os.Stderr, "Received untyped session update: %+v\n", params.Update)
		return nil
	}

	handleTypedSessionUpdate(update)
	return nil
}

// handleTypedSessionUpdate processes typed session updates.
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
		fmt.Fprintf(os.Stderr, "Unknown session update type: %s\n", update.Type)
	}
}

// handleAgentMessageChunk processes agent message chunks.
func handleAgentMessageChunk(chunk *api.SessionUpdateAgentMessageChunk) {
	if chunk == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Agent message chunk received\n")
	if textContent := chunk.Content.GetText(); textContent != nil {
		fmt.Fprintf(os.Stderr, "Text content: %s\n", textContent.Text)
	}
}

// handleAgentThoughtChunk processes agent thought chunks.
func handleAgentThoughtChunk(chunk *api.SessionUpdateAgentThoughtChunk) {
	if chunk == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Agent thought chunk received\n")
	if textContent := chunk.Content.GetText(); textContent != nil {
		fmt.Fprintf(os.Stderr, "Thought content: %s\n", textContent.Text)
	}
}

// handlePlanUpdate processes plan updates.
func handlePlanUpdate(plan *api.SessionUpdatePlan) {
	if plan == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Agent plan received with %d entries\n", len(plan.Entries))
}

// handleToolCallUpdate processes tool call notifications.
func handleToolCallUpdate(toolCall *api.SessionUpdateToolCall) {
	if toolCall == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Tool call received: %s\n", toolCall.Title)
	if toolCall.Kind != nil && toolCall.Status != nil {
		fmt.Fprintf(os.Stderr, "Tool call kind: %s, status: %s\n", *toolCall.Kind, *toolCall.Status)
	}
}

// handleToolCallUpdateNotification processes tool call update notifications.
func handleToolCallUpdateNotification(toolCallUpdate *api.SessionUpdateToolCallUpdate) {
	if toolCallUpdate == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Tool call update received for: %s\n", toolCallUpdate.Title)
}

// handleUserMessageChunk processes user message chunks.
func handleUserMessageChunk(chunk *api.SessionUpdateUserMessageChunk) {
	if chunk == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "User message chunk received\n")
}
