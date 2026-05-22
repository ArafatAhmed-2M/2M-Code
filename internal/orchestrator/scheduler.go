// Package orchestrator provides turn scheduling for agent teams.
//
// The scheduler determines the order in which agents take turns based on the
// team's workflow configuration:
//
//   - leader_first: Leader speaks first, then workers, then reviewer last
//   - round_robin:  All agents take equal turns in order
//   - free:         Same as round_robin (reserved for future use)
package orchestrator

import (
	"github.com/2mcode/2mcode/internal/team"
)

// BuildSchedule creates the ordered list of agent names for a complete task.
//
// For leader_first orchestration with TurnsPerTask=2 and agents [Aria, Dev, Quinn]:
//   - Aria (leader) speaks first
//   - Round 1: Dev
//   - Round 2: Dev
//   - Quinn (reviewer) speaks last
//
// For round_robin with TurnsPerTask=2 and agents [Aria, Dev, Quinn]:
//   - Round 1: Aria, Dev, Quinn
//   - Round 2: Aria, Dev, Quinn
func BuildSchedule(t *team.Team) []string {
	switch t.Workflow.Orchestration {
	case "leader_first":
		return buildLeaderFirstSchedule(t)
	case "round_robin", "free":
		return buildRoundRobinSchedule(t)
	default:
		// Fall back to leader_first if leader is defined, otherwise round_robin
		if t.Workflow.Leader != "" {
			return buildLeaderFirstSchedule(t)
		}
		return buildRoundRobinSchedule(t)
	}
}

// buildLeaderFirstSchedule creates the turn order for leader-first orchestration:
//   1. Leader speaks first
//   2. Worker agents take turns for TurnsPerTask rounds
//   3. Reviewer speaks last (if defined)
func buildLeaderFirstSchedule(t *team.Team) []string {
	var schedule []string

	// Identify workers (non-leader, non-reviewer agents)
	var workers []string
	for _, agent := range t.Agents {
		if agent.Name == t.Workflow.Leader {
			continue
		}
		if agent.Name == t.Workflow.Reviewer {
			continue
		}
		workers = append(workers, agent.Name)
	}

	// 1. Leader speaks first
	if t.Workflow.Leader != "" {
		schedule = append(schedule, t.Workflow.Leader)
	}

	// 2. Workers take turns for TurnsPerTask rounds
	for round := 0; round < t.Workflow.TurnsPerTask; round++ {
		for _, worker := range workers {
			schedule = append(schedule, worker)
		}
	}

	// 3. Reviewer speaks last
	if t.Workflow.Reviewer != "" {
		schedule = append(schedule, t.Workflow.Reviewer)
	}

	return schedule
}

// buildRoundRobinSchedule creates the turn order for round-robin orchestration:
// All agents take turns in order, repeated for TurnsPerTask rounds.
func buildRoundRobinSchedule(t *team.Team) []string {
	var schedule []string

	for round := 0; round < t.Workflow.TurnsPerTask; round++ {
		for _, agent := range t.Agents {
			schedule = append(schedule, agent.Name)
		}
	}

	return schedule
}

// GetAgentTurnCount returns how many times a specific agent appears in the schedule.
func GetAgentTurnCount(schedule []string, agentName string) int {
	count := 0
	for _, name := range schedule {
		if name == agentName {
			count++
		}
	}
	return count
}
