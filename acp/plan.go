package acp

import (
	"context"
	"errors"
	"fmt"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
)

// PlanBuilder provides a fluent interface for building execution plans.
//
// Plans are strategies that agents share with clients through session updates,
// providing real-time visibility into their thinking and progress.
type PlanBuilder struct {
	entries []api.PlanEntry
}

// NewPlanBuilder creates a new plan builder.
func NewPlanBuilder() *PlanBuilder {
	return &PlanBuilder{
		entries: make([]api.PlanEntry, 0),
	}
}

// AddEntry adds a new entry to the plan with the specified content and priority.
func (pb *PlanBuilder) AddEntry(content string, priority api.PlanEntryPriority) *PlanBuilder {
	entry := api.PlanEntry{
		Content:  content,
		Priority: priority,
		Status:   api.PlanEntryStatusPending,
	}
	pb.entries = append(pb.entries, entry)
	return pb
}

// AddHighPriorityEntry adds a high priority entry to the plan.
func (pb *PlanBuilder) AddHighPriorityEntry(content string) *PlanBuilder {
	return pb.AddEntry(content, api.PlanEntryPriorityHigh)
}

// AddMediumPriorityEntry adds a medium priority entry to the plan.
func (pb *PlanBuilder) AddMediumPriorityEntry(content string) *PlanBuilder {
	return pb.AddEntry(content, api.PlanEntryPriorityMedium)
}

// AddLowPriorityEntry adds a low priority entry to the plan.
func (pb *PlanBuilder) AddLowPriorityEntry(content string) *PlanBuilder {
	return pb.AddEntry(content, api.PlanEntryPriorityLow)
}

// AddEntries adds multiple entries with the same priority.
func (pb *PlanBuilder) AddEntries(contents []string, priority api.PlanEntryPriority) *PlanBuilder {
	for _, content := range contents {
		pb.AddEntry(content, priority)
	}
	return pb
}

// Build creates the final Plan from the builder.
func (pb *PlanBuilder) Build() *api.Plan {
	// Make a copy of entries to avoid external mutation
	entries := make([]api.PlanEntry, len(pb.entries))
	copy(entries, pb.entries)

	return &api.Plan{
		Entries: entries,
	}
}

// Reset clears all entries from the builder, allowing it to be reused.
func (pb *PlanBuilder) Reset() *PlanBuilder {
	pb.entries = pb.entries[:0]
	return pb
}

// EntryCount returns the current number of entries in the builder.
func (pb *PlanBuilder) EntryCount() int {
	return len(pb.entries)
}

// Plan management utilities

// UpdatePlanEntry updates a plan entry at the specified index with a new status.
func UpdatePlanEntry(plan *api.Plan, index int, status api.PlanEntryStatus) error {
	if plan == nil {
		return errors.New("plan cannot be nil")
	}
	if index < 0 || index >= len(plan.Entries) {
		return fmt.Errorf("index %d out of range [0, %d)", index, len(plan.Entries))
	}

	plan.Entries[index].Status = status
	return nil
}

// UpdatePlanEntryByContent updates the first plan entry with matching content to the new status.
func UpdatePlanEntryByContent(plan *api.Plan, content string, status api.PlanEntryStatus) error {
	if plan == nil {
		return errors.New("plan cannot be nil")
	}

	for i, entry := range plan.Entries {
		if entry.Content == content {
			plan.Entries[i].Status = status
			return nil
		}
	}

	return fmt.Errorf("plan entry with content %q not found", content)
}

// FindPlanEntry finds the first plan entry with matching content.
func FindPlanEntry(plan *api.Plan, content string) *api.PlanEntry {
	if plan == nil {
		return nil
	}

	for i, entry := range plan.Entries {
		if entry.Content == content {
			return &plan.Entries[i]
		}
	}

	return nil
}

// FindPlanEntryIndex finds the index of the first plan entry with matching content.
func FindPlanEntryIndex(plan *api.Plan, content string) int {
	if plan == nil {
		return -1
	}

	for i, entry := range plan.Entries {
		if entry.Content == content {
			return i
		}
	}

	return -1
}

// GetPlanEntriesByStatus returns all plan entries with the specified status.
func GetPlanEntriesByStatus(plan *api.Plan, status api.PlanEntryStatus) []api.PlanEntry {
	if plan == nil {
		return nil
	}

	var result []api.PlanEntry
	for _, entry := range plan.Entries {
		if entry.Status == status {
			result = append(result, entry)
		}
	}

	return result
}

// GetPlanEntriesByPriority returns all plan entries with the specified priority.
func GetPlanEntriesByPriority(plan *api.Plan, priority api.PlanEntryPriority) []api.PlanEntry {
	if plan == nil {
		return nil
	}

	var result []api.PlanEntry
	for _, entry := range plan.Entries {
		if entry.Priority == priority {
			result = append(result, entry)
		}
	}

	return result
}

// GetPlanProgress returns the progress of the plan as completed/total entries.
func GetPlanProgress(plan *api.Plan) (int, int) {
	if plan == nil {
		return 0, 0
	}

	completed := 0
	total := len(plan.Entries)
	for _, entry := range plan.Entries {
		if entry.Status == api.PlanEntryStatusCompleted {
			completed++
		}
	}

	return completed, total
}

// IsPlanComplete returns true if all entries in the plan are completed.
func IsPlanComplete(plan *api.Plan) bool {
	completed, total := GetPlanProgress(plan)
	return total > 0 && completed == total
}

// GetNextPendingEntry returns the first pending entry in the plan, or nil if none.
func GetNextPendingEntry(plan *api.Plan) *api.PlanEntry {
	entries := GetPlanEntriesByStatus(plan, api.PlanEntryStatusPending)
	if len(entries) > 0 {
		return &entries[0]
	}
	return nil
}

// GetInProgressEntries returns all entries currently in progress.
func GetInProgressEntries(plan *api.Plan) []api.PlanEntry {
	return GetPlanEntriesByStatus(plan, api.PlanEntryStatusInProgress)
}

// ValidatePlan performs basic validation on a plan.
func ValidatePlan(plan *api.Plan) error {
	if plan == nil {
		return errors.New("plan cannot be nil")
	}

	if len(plan.Entries) == 0 {
		return errors.New("plan must have at least one entry")
	}

	for i, entry := range plan.Entries {
		if entry.Content == "" {
			return fmt.Errorf("entry %d has empty content", i)
		}

		// Validate priority
		switch entry.Priority {
		case api.PlanEntryPriorityHigh, api.PlanEntryPriorityMedium, api.PlanEntryPriorityLow:
			// Valid priority
		default:
			return fmt.Errorf("entry %d has invalid priority: %v", i, entry.Priority)
		}

		// Validate status
		switch entry.Status {
		case api.PlanEntryStatusPending, api.PlanEntryStatusInProgress, api.PlanEntryStatusCompleted:
			// Valid status
		default:
			return fmt.Errorf("entry %d has invalid status: %v", i, entry.Status)
		}
	}

	return nil
}

// Integration with AgentConnection

// SendPlanUpdate sends a plan update through the agent connection.
func (a *AgentConnection) SendPlanUpdate(ctx context.Context, sessionID api.SessionId, plan *api.Plan) error {
	if err := ValidatePlan(plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}

	// Convert plan entries to []interface{} as expected by the API
	entries := make([]interface{}, len(plan.Entries))
	for i, entry := range plan.Entries {
		entries[i] = entry
	}

	update := api.NewSessionUpdatePlan(entries)
	return a.SendSessionUpdate(ctx, &api.SessionNotification{
		SessionId: sessionID,
		Update:    update,
	})
}

// Plan workflow helpers

// PlanWorkflow helps manage a plan's execution workflow.
type PlanWorkflow struct {
	plan      *api.Plan
	sessionID api.SessionId
	conn      *AgentConnection
}

// NewPlanWorkflow creates a new plan workflow manager.
func NewPlanWorkflow(plan *api.Plan, sessionID api.SessionId, conn *AgentConnection) (*PlanWorkflow, error) {
	if err := ValidatePlan(plan); err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}

	return &PlanWorkflow{
		plan:      plan,
		sessionID: sessionID,
		conn:      conn,
	}, nil
}

// StartEntry marks an entry as in progress and sends an update.
func (pw *PlanWorkflow) StartEntry(ctx context.Context, content string) error {
	err := UpdatePlanEntryByContent(pw.plan, content, api.PlanEntryStatusInProgress)
	if err != nil {
		return err
	}

	return pw.conn.SendPlanUpdate(ctx, pw.sessionID, pw.plan)
}

// CompleteEntry marks an entry as completed and sends an update.
func (pw *PlanWorkflow) CompleteEntry(ctx context.Context, content string) error {
	err := UpdatePlanEntryByContent(pw.plan, content, api.PlanEntryStatusCompleted)
	if err != nil {
		return err
	}

	return pw.conn.SendPlanUpdate(ctx, pw.sessionID, pw.plan)
}

// StartEntryByIndex marks an entry at the given index as in progress and sends an update.
func (pw *PlanWorkflow) StartEntryByIndex(ctx context.Context, index int) error {
	err := UpdatePlanEntry(pw.plan, index, api.PlanEntryStatusInProgress)
	if err != nil {
		return err
	}

	return pw.conn.SendPlanUpdate(ctx, pw.sessionID, pw.plan)
}

// CompleteEntryByIndex marks an entry at the given index as completed and sends an update.
func (pw *PlanWorkflow) CompleteEntryByIndex(ctx context.Context, index int) error {
	err := UpdatePlanEntry(pw.plan, index, api.PlanEntryStatusCompleted)
	if err != nil {
		return err
	}

	return pw.conn.SendPlanUpdate(ctx, pw.sessionID, pw.plan)
}

// GetPlan returns the current plan.
func (pw *PlanWorkflow) GetPlan() *api.Plan {
	return pw.plan
}

// IsComplete returns true if all plan entries are completed.
func (pw *PlanWorkflow) IsComplete() bool {
	return IsPlanComplete(pw.plan)
}

// GetProgress returns the current progress as completed/total entries.
func (pw *PlanWorkflow) GetProgress() (int, int) {
	return GetPlanProgress(pw.plan)
}

// Convenience functions for common plan patterns

// CreateSimplePlan creates a plan with a list of tasks, all at medium priority.
func CreateSimplePlan(tasks []string) *api.Plan {
	builder := NewPlanBuilder()
	for _, task := range tasks {
		builder.AddMediumPriorityEntry(task)
	}
	return builder.Build()
}

// CreatePrioritizedPlan creates a plan with high priority tasks first, then medium, then low.
func CreatePrioritizedPlan(highTasks, mediumTasks, lowTasks []string) *api.Plan {
	builder := NewPlanBuilder()
	builder.AddEntries(highTasks, api.PlanEntryPriorityHigh)
	builder.AddEntries(mediumTasks, api.PlanEntryPriorityMedium)
	builder.AddEntries(lowTasks, api.PlanEntryPriorityLow)
	return builder.Build()
}
