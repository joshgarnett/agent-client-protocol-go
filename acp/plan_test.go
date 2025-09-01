package acp

import (
	"context"
	"testing"

	"github.com/joshgarnett/agent-client-protocol-go/acp/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanBuilder(t *testing.T) {
	t.Run("Create Simple Plan", func(t *testing.T) {
		builder := NewPlanBuilder()
		builder.
			AddEntry("Task 1", api.PlanEntryPriorityHigh).
			AddEntry("Task 2", api.PlanEntryPriorityMedium).
			AddEntry("Task 3", api.PlanEntryPriorityLow)

		plan := builder.Build()

		assert.Len(t, plan.Entries, 3)
		assert.Equal(t, "Task 1", plan.Entries[0].Content)
		assert.Equal(t, api.PlanEntryPriorityHigh, plan.Entries[0].Priority)
		assert.Equal(t, api.PlanEntryStatusPending, plan.Entries[0].Status)
	})

	t.Run("Add Priority Entries", func(t *testing.T) {
		builder := NewPlanBuilder()
		builder.
			AddHighPriorityEntry("High task").
			AddMediumPriorityEntry("Medium task").
			AddLowPriorityEntry("Low task")

		plan := builder.Build()

		assert.Len(t, plan.Entries, 3)
		assert.Equal(t, api.PlanEntryPriorityHigh, plan.Entries[0].Priority)
		assert.Equal(t, api.PlanEntryPriorityMedium, plan.Entries[1].Priority)
		assert.Equal(t, api.PlanEntryPriorityLow, plan.Entries[2].Priority)
	})

	t.Run("Add Multiple Entries", func(t *testing.T) {
		builder := NewPlanBuilder()
		tasks := []string{"Task A", "Task B", "Task C"}
		builder.AddEntries(tasks, api.PlanEntryPriorityMedium)

		plan := builder.Build()

		assert.Len(t, plan.Entries, 3)
		for i, task := range tasks {
			assert.Equal(t, task, plan.Entries[i].Content)
			assert.Equal(t, api.PlanEntryPriorityMedium, plan.Entries[i].Priority)
		}
	})

	t.Run("Reset Builder", func(t *testing.T) {
		builder := NewPlanBuilder()
		builder.AddEntry("Task 1", api.PlanEntryPriorityHigh)

		assert.Equal(t, 1, builder.EntryCount())

		builder.Reset()
		assert.Equal(t, 0, builder.EntryCount())

		// Can reuse after reset
		builder.AddEntry("Task 2", api.PlanEntryPriorityLow)
		plan := builder.Build()
		assert.Len(t, plan.Entries, 1)
		assert.Equal(t, "Task 2", plan.Entries[0].Content)
	})

	t.Run("EntryCount", func(t *testing.T) {
		builder := NewPlanBuilder()
		assert.Equal(t, 0, builder.EntryCount())

		builder.AddEntry("Task 1", api.PlanEntryPriorityHigh)
		assert.Equal(t, 1, builder.EntryCount())

		builder.AddEntries([]string{"Task 2", "Task 3"}, api.PlanEntryPriorityMedium)
		assert.Equal(t, 3, builder.EntryCount())
	})
}

func TestPlanManagement(t *testing.T) {
	t.Run("UpdatePlanEntry", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh, Status: api.PlanEntryStatusPending},
				{Content: "Task 2", Priority: api.PlanEntryPriorityMedium, Status: api.PlanEntryStatusPending},
			},
		}

		err := UpdatePlanEntry(plan, 0, api.PlanEntryStatusInProgress)
		require.NoError(t, err)
		assert.Equal(t, api.PlanEntryStatusInProgress, plan.Entries[0].Status)

		err = UpdatePlanEntry(plan, 1, api.PlanEntryStatusCompleted)
		require.NoError(t, err)
		assert.Equal(t, api.PlanEntryStatusCompleted, plan.Entries[1].Status)
	})

	t.Run("UpdatePlanEntry Errors", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh, Status: api.PlanEntryStatusPending},
			},
		}

		// Nil plan
		err := UpdatePlanEntry(nil, 0, api.PlanEntryStatusInProgress)
		require.Error(t, err)

		// Index out of range
		err = UpdatePlanEntry(plan, -1, api.PlanEntryStatusInProgress)
		require.Error(t, err)

		err = UpdatePlanEntry(plan, 5, api.PlanEntryStatusInProgress)
		require.Error(t, err)
	})

	t.Run("UpdatePlanEntryByContent", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh, Status: api.PlanEntryStatusPending},
				{Content: "Task 2", Priority: api.PlanEntryPriorityMedium, Status: api.PlanEntryStatusPending},
			},
		}

		err := UpdatePlanEntryByContent(plan, "Task 1", api.PlanEntryStatusInProgress)
		require.NoError(t, err)
		assert.Equal(t, api.PlanEntryStatusInProgress, plan.Entries[0].Status)

		// Not found
		err = UpdatePlanEntryByContent(plan, "Task 3", api.PlanEntryStatusCompleted)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("FindPlanEntry", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh, Status: api.PlanEntryStatusPending},
				{Content: "Task 2", Priority: api.PlanEntryPriorityMedium, Status: api.PlanEntryStatusInProgress},
			},
		}

		entry := FindPlanEntry(plan, "Task 2")
		require.NotNil(t, entry)
		assert.Equal(t, "Task 2", entry.Content)
		assert.Equal(t, api.PlanEntryStatusInProgress, entry.Status)

		entry = FindPlanEntry(plan, "Task 3")
		assert.Nil(t, entry)

		entry = FindPlanEntry(nil, "Task 1")
		assert.Nil(t, entry)
	})

	t.Run("FindPlanEntryIndex", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh},
				{Content: "Task 2", Priority: api.PlanEntryPriorityMedium},
			},
		}

		index := FindPlanEntryIndex(plan, "Task 1")
		assert.Equal(t, 0, index)

		index = FindPlanEntryIndex(plan, "Task 2")
		assert.Equal(t, 1, index)

		index = FindPlanEntryIndex(plan, "Task 3")
		assert.Equal(t, -1, index)

		index = FindPlanEntryIndex(nil, "Task 1")
		assert.Equal(t, -1, index)
	})
}

func TestPlanQueries(t *testing.T) {
	t.Run("GetPlanEntriesByStatus", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Status: api.PlanEntryStatusPending},
				{Content: "Task 2", Status: api.PlanEntryStatusInProgress},
				{Content: "Task 3", Status: api.PlanEntryStatusCompleted},
				{Content: "Task 4", Status: api.PlanEntryStatusPending},
			},
		}

		pending := GetPlanEntriesByStatus(plan, api.PlanEntryStatusPending)
		assert.Len(t, pending, 2)
		assert.Equal(t, "Task 1", pending[0].Content)
		assert.Equal(t, "Task 4", pending[1].Content)

		inProgress := GetPlanEntriesByStatus(plan, api.PlanEntryStatusInProgress)
		assert.Len(t, inProgress, 1)
		assert.Equal(t, "Task 2", inProgress[0].Content)

		completed := GetPlanEntriesByStatus(plan, api.PlanEntryStatusCompleted)
		assert.Len(t, completed, 1)
		assert.Equal(t, "Task 3", completed[0].Content)

		result := GetPlanEntriesByStatus(nil, api.PlanEntryStatusPending)
		assert.Nil(t, result)
	})

	t.Run("GetPlanEntriesByPriority", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Priority: api.PlanEntryPriorityHigh},
				{Content: "Task 2", Priority: api.PlanEntryPriorityMedium},
				{Content: "Task 3", Priority: api.PlanEntryPriorityHigh},
				{Content: "Task 4", Priority: api.PlanEntryPriorityLow},
			},
		}

		high := GetPlanEntriesByPriority(plan, api.PlanEntryPriorityHigh)
		assert.Len(t, high, 2)

		medium := GetPlanEntriesByPriority(plan, api.PlanEntryPriorityMedium)
		assert.Len(t, medium, 1)

		low := GetPlanEntriesByPriority(plan, api.PlanEntryPriorityLow)
		assert.Len(t, low, 1)

		result := GetPlanEntriesByPriority(nil, api.PlanEntryPriorityHigh)
		assert.Nil(t, result)
	})

	t.Run("GetPlanProgress", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Status: api.PlanEntryStatusCompleted},
				{Content: "Task 2", Status: api.PlanEntryStatusInProgress},
				{Content: "Task 3", Status: api.PlanEntryStatusCompleted},
				{Content: "Task 4", Status: api.PlanEntryStatusPending},
			},
		}

		completed, total := GetPlanProgress(plan)
		assert.Equal(t, 2, completed)
		assert.Equal(t, 4, total)

		completed, total = GetPlanProgress(nil)
		assert.Equal(t, 0, completed)
		assert.Equal(t, 0, total)
	})

	t.Run("IsPlanComplete", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Status: api.PlanEntryStatusCompleted},
				{Content: "Task 2", Status: api.PlanEntryStatusCompleted},
			},
		}

		assert.True(t, IsPlanComplete(plan))

		plan.Entries[1].Status = api.PlanEntryStatusPending
		assert.False(t, IsPlanComplete(plan))

		assert.False(t, IsPlanComplete(nil))
		assert.False(t, IsPlanComplete(&api.Plan{}))
	})

	t.Run("GetNextPendingEntry", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Status: api.PlanEntryStatusCompleted},
				{Content: "Task 2", Status: api.PlanEntryStatusPending},
				{Content: "Task 3", Status: api.PlanEntryStatusPending},
			},
		}

		next := GetNextPendingEntry(plan)
		require.NotNil(t, next)
		assert.Equal(t, "Task 2", next.Content)

		// All completed
		plan.Entries[1].Status = api.PlanEntryStatusCompleted
		plan.Entries[2].Status = api.PlanEntryStatusCompleted
		next = GetNextPendingEntry(plan)
		assert.Nil(t, next)
	})

	t.Run("GetInProgressEntries", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{Content: "Task 1", Status: api.PlanEntryStatusInProgress},
				{Content: "Task 2", Status: api.PlanEntryStatusPending},
				{Content: "Task 3", Status: api.PlanEntryStatusInProgress},
			},
		}

		inProgress := GetInProgressEntries(plan)
		assert.Len(t, inProgress, 2)
		assert.Equal(t, "Task 1", inProgress[0].Content)
		assert.Equal(t, "Task 3", inProgress[1].Content)
	})
}

func TestValidatePlan(t *testing.T) {
	t.Run("Valid Plan", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{
					Content:  "Task 1",
					Priority: api.PlanEntryPriorityHigh,
					Status:   api.PlanEntryStatusPending,
				},
			},
		}

		err := ValidatePlan(plan)
		assert.NoError(t, err)
	})

	t.Run("Nil Plan", func(t *testing.T) {
		err := ValidatePlan(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("Empty Plan", func(t *testing.T) {
		plan := &api.Plan{}
		err := ValidatePlan(plan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one entry")
	})

	t.Run("Empty Content", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{
					Content:  "",
					Priority: api.PlanEntryPriorityHigh,
					Status:   api.PlanEntryStatusPending,
				},
			},
		}

		err := ValidatePlan(plan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty content")
	})

	t.Run("Invalid Priority", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{
					Content:  "Task 1",
					Priority: api.PlanEntryPriority("invalid"),
					Status:   api.PlanEntryStatusPending,
				},
			},
		}

		err := ValidatePlan(plan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid priority")
	})

	t.Run("Invalid Status", func(t *testing.T) {
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{
					Content:  "Task 1",
					Priority: api.PlanEntryPriorityHigh,
					Status:   api.PlanEntryStatus("invalid"),
				},
			},
		}

		err := ValidatePlan(plan)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status")
	})
}

func TestPlanWorkflow(t *testing.T) {
	t.Run("Workflow Operations", func(t *testing.T) {
		// Create a mock connection
		plan := CreateSimplePlan([]string{"Task 1", "Task 2", "Task 3"})

		// Create workflow (would need mock connection in real test)
		// For now, just test the plan creation
		assert.Len(t, plan.Entries, 3)
		for _, entry := range plan.Entries {
			assert.Equal(t, api.PlanEntryPriorityMedium, entry.Priority)
			assert.Equal(t, api.PlanEntryStatusPending, entry.Status)
		}
	})

	t.Run("CreatePrioritizedPlan", func(t *testing.T) {
		high := []string{"Critical 1", "Critical 2"}
		medium := []string{"Normal 1", "Normal 2", "Normal 3"}
		low := []string{"Low priority 1"}

		plan := CreatePrioritizedPlan(high, medium, low)

		assert.Len(t, plan.Entries, 6)

		// Check high priority tasks
		assert.Equal(t, api.PlanEntryPriorityHigh, plan.Entries[0].Priority)
		assert.Equal(t, api.PlanEntryPriorityHigh, plan.Entries[1].Priority)

		// Check medium priority tasks
		assert.Equal(t, api.PlanEntryPriorityMedium, plan.Entries[2].Priority)
		assert.Equal(t, api.PlanEntryPriorityMedium, plan.Entries[3].Priority)
		assert.Equal(t, api.PlanEntryPriorityMedium, plan.Entries[4].Priority)

		// Check low priority task
		assert.Equal(t, api.PlanEntryPriorityLow, plan.Entries[5].Priority)
	})
}

// MockAgentConnection for testing SendPlanUpdate.
type MockAgentConnection struct {
	lastUpdate *api.SessionNotification
}

func (m *MockAgentConnection) SendSessionUpdate(_ context.Context, notif *api.SessionNotification) error {
	m.lastUpdate = notif
	return nil
}

func TestSendPlanUpdate(t *testing.T) {
	t.Run("Valid Plan Update", func(t *testing.T) {
		// This would need a proper mock of AgentConnection
		// For now, we test the validation part
		plan := &api.Plan{
			Entries: []api.PlanEntry{
				{
					Content:  "Task 1",
					Priority: api.PlanEntryPriorityHigh,
					Status:   api.PlanEntryStatusPending,
				},
			},
		}

		err := ValidatePlan(plan)
		assert.NoError(t, err)
	})
}
