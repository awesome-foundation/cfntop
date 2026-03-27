package tui

import (
	"context"
	"sort"
	"strings"
	"time"

	cfnaws "github.com/awesome-foundation/cfntop/internal/aws"
	"github.com/awesome-foundation/cfntop/internal/model"
)

// ResourceState holds a resource and its recent events from the most recent deploy.
type ResourceState struct {
	Resource model.Resource
	Events   []model.StackEvent
	HasError bool
	Touched  bool             // resource has events from the most recent deploy
	Deleted  bool             // resource was deleted in a recent deploy
	ECS      *ECSServiceState // non-nil for AWS::ECS::Service resources
}

// StackState holds a stack's summary, resources, and events.
type StackState struct {
	Summary   model.StackSummary
	Resources []ResourceState
	Expanded  bool
	Active    bool
}

// PollResult is the result of a single poll cycle.
type PollResult struct {
	Stacks []StackState
	Err    error
}

func isActive(status string) bool {
	for _, suffix := range []string{"IN_PROGRESS", "CLEANUP_IN_PROGRESS"} {
		if len(status) >= len(suffix) && status[len(status)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}

func isFailed(status string) bool {
	return strings.Contains(status, "FAILED")
}

// recentDeleteCutoff is how far back we look for deleted resources.
const recentDeleteCutoff = 24 * time.Hour

// deployStartStatuses are the statuses that mark the beginning of a new deploy.
// These are the initial user-triggered or API-triggered status changes on the stack itself.
var deployStartStatuses = map[string]bool{
	"CREATE_IN_PROGRESS": true,
	"UPDATE_IN_PROGRESS": true,
	"DELETE_IN_PROGRESS": true,
	"IMPORT_IN_PROGRESS": true,
}

// mostRecentDeployEvents returns only events from the most recent deploy.
// A deploy starts with a specific *_IN_PROGRESS event on the stack itself (not cleanup/rollback).
// Events are assumed to be in ascending timestamp order (oldest first).
func mostRecentDeployEvents(events []model.StackEvent, stackName string) []model.StackEvent {
	// Walk backwards to find the most recent deploy start
	lastDeployStart := -1
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if e.LogicalID == stackName && deployStartStatuses[e.Status] {
			lastDeployStart = i
			break
		}
	}
	if lastDeployStart < 0 {
		return events // no boundary found, return all
	}
	return events[lastDeployStart:]
}

// buildResourceStates combines resources with their events from the most recent deploy,
// and includes recently-deleted resources from events.
func buildResourceStates(resources []model.Resource, events []model.StackEvent, stackName string) []ResourceState {
	// Filter to only events from the most recent deploy
	deployEvents := mostRecentDeployEvents(events, stackName)

	// Group events by logical ID
	eventsByID := make(map[string][]model.StackEvent)
	for _, e := range deployEvents {
		eventsByID[e.LogicalID] = append(eventsByID[e.LogicalID], e)
	}

	// Track which logical IDs are in the current resource list
	currentIDs := make(map[string]bool)
	for _, r := range resources {
		currentIDs[r.LogicalID] = true
	}

	var states []ResourceState

	// Current resources
	for _, r := range resources {
		rs := ResourceState{Resource: r}
		rs.Events = eventsByID[r.LogicalID]
		rs.Touched = len(rs.Events) > 0
		for _, e := range rs.Events {
			if isFailed(e.Status) {
				rs.HasError = true
				break
			}
		}
		states = append(states, rs)
	}

	// Recently deleted resources (from events, not in current resources)
	now := time.Now()
	seen := make(map[string]bool)
	for _, e := range events {
		if currentIDs[e.LogicalID] || seen[e.LogicalID] {
			continue
		}
		if e.Status != "DELETE_COMPLETE" {
			continue
		}
		t, err := time.Parse("2006-01-02T15:04:05Z", e.Timestamp)
		if err != nil {
			continue
		}
		if now.Sub(t) > recentDeleteCutoff {
			continue
		}
		seen[e.LogicalID] = true
		states = append(states, ResourceState{
			Resource: model.Resource{
				LogicalID:   e.LogicalID,
				Type:        e.ResourceType,
				Status:      "DELETE_COMPLETE",
				LastUpdated: e.Timestamp,
			},
			Events:  eventsByID[e.LogicalID],
			Deleted: true,
		})
	}

	// Sort: errors first, then active, then by most recent update descending
	sort.Slice(states, func(i, j int) bool {
		ri, rj := states[i], states[j]
		// Errors first
		if ri.HasError != rj.HasError {
			return ri.HasError
		}
		// Active before complete/deleted
		iActive := isActive(ri.Resource.Status)
		jActive := isActive(rj.Resource.Status)
		if iActive != jActive {
			return iActive
		}
		// Most recently updated first
		return ri.Resource.LastUpdated > rj.Resource.LastUpdated
	})

	return states
}

// FetchStackDetails fetches resources, recent events, and ECS details for a stack.
func FetchStackDetails(client *cfnaws.Client, ecsClient ECSAPI, stackName string) ([]ResourceState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resources, err := client.FetchResources(ctx, stackName)
	if err != nil {
		return nil, err
	}

	// Fetch last 50 events to get the most recent deploy
	events, err := client.FetchEvents(ctx, stackName, 50)
	if err != nil {
		// Non-fatal — show resources without events
		return enrichECS(ctx, ecsClient, buildResourceStates(resources, nil, stackName)), nil
	}

	return enrichECS(ctx, ecsClient, buildResourceStates(resources, events.Events, stackName)), nil
}

// enrichECS fetches ECS deployment details for any AWS::ECS::Service resources.
func enrichECS(ctx context.Context, ecsClient ECSAPI, states []ResourceState) []ResourceState {
	if ecsClient == nil {
		return states
	}
	for i := range states {
		if states[i].Resource.Type != "AWS::ECS::Service" {
			continue
		}
		if states[i].Resource.PhysicalID == "" {
			continue
		}
		ecsState, err := FetchECSServiceState(ctx, ecsClient, states[i].Resource.PhysicalID)
		if err == nil {
			states[i].ECS = ecsState
		}
	}
	return states
}

// Poll fetches all stacks. expandedStacks lists stack names that are currently expanded in the UI.
func Poll(client *cfnaws.Client, ecsClient ECSAPI, expandedStacks map[string]bool) PollResult {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	list, err := client.FetchStacks(ctx)
	if err != nil {
		return PollResult{Err: err}
	}

	var stacks []StackState
	for _, s := range list.Stacks {
		state := StackState{
			Summary: s,
			Active:  isActive(s.Status),
		}

		// Fetch details for active stacks and expanded stacks
		if state.Active || expandedStacks[s.StackName] {
			rs, err := FetchStackDetails(client, ecsClient, s.StackName)
			if err == nil {
				state.Resources = rs
			}
		}

		stacks = append(stacks, state)
	}

	// Sort: active first, then by last update descending
	sort.Slice(stacks, func(i, j int) bool {
		if stacks[i].Active != stacks[j].Active {
			return stacks[i].Active
		}
		ti := stacks[i].Summary.UpdatedAt
		if ti == "" {
			ti = stacks[i].Summary.CreatedAt
		}
		tj := stacks[j].Summary.UpdatedAt
		if tj == "" {
			tj = stacks[j].Summary.CreatedAt
		}
		return ti > tj
	})

	return PollResult{Stacks: stacks}
}
