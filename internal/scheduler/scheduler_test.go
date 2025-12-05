package scheduler

import (
	"testing"

	"github.com/bit2swaz/orion/internal/task"
)

func TestSelectCandidate(t *testing.T) {
	sched := New()

	tests := []struct {
		name     string
		task     task.Task
		nodes    []Node
		wantNode string
	}{
		{
			name: "Basic Fit",
			task: task.Task{Memory: 100, Disk: 100},
			nodes: []Node{
				{ID: "node1", MemoryTotal: 1000, DiskTotal: 1000},
			},
			wantNode: "node1",
		},
		{
			name: "Resource Constraint (Not Enough RAM)",
			task: task.Task{Memory: 500},
			nodes: []Node{
				{ID: "node1", MemoryTotal: 1000, MemoryUsed: 600},
			},
			wantNode: "",
		},
		{
			name: "Selector Mismatch",
			task: task.Task{NodeSelectors: map[string]string{"gpu": "true"}},
			nodes: []Node{
				{ID: "node1", MemoryTotal: 1000, Tags: map[string]string{"gpu": "false"}},
			},
			wantNode: "",
		},
		{
			name: "Best Score (Bin Packing)",
			task: task.Task{Memory: 100},
			nodes: []Node{
				{ID: "node-small", MemoryTotal: 1000, MemoryUsed: 800},
				{ID: "node-big", MemoryTotal: 1000, MemoryUsed: 100},
			},
			wantNode: "node-big",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sched.SelectCandidate(tt.task, tt.nodes)

			if tt.wantNode == "" {
				if got != nil {
					t.Errorf("expected nil, got %s", got.ID)
				}
			} else {
				if got == nil {
					t.Errorf("expected %s, got nil", tt.wantNode)
				} else if got.ID != tt.wantNode {
					t.Errorf("expected %s, got %s", tt.wantNode, got.ID)
				}
			}
		})
	}
}
