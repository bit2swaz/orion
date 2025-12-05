package scheduler

import (
	"github.com/bit2swaz/orion/internal/task"
)

type Node struct {
	ID          string
	MemoryTotal int64
	MemoryUsed  int64
	DiskTotal   int64
	DiskUsed    int64
	Tags        map[string]string
}

type Scheduler struct{}

func New() *Scheduler {
	return &Scheduler{}
}

func (s *Scheduler) SelectCandidate(t task.Task, nodes []Node) *Node {
	var bestNode *Node
	var maxScore int64 = -1

	for i, node := range nodes {
		freeMemory := node.MemoryTotal - node.MemoryUsed
		freeDisk := node.DiskTotal - node.DiskUsed

		if freeDisk < t.Disk {
			continue
		}
		if freeMemory < t.Memory {
			continue
		}

		matchesSelectors := true
		for k, v := range t.NodeSelectors {
			if nodeVal, ok := node.Tags[k]; !ok || nodeVal != v {
				matchesSelectors = false
				break
			}
		}
		if !matchesSelectors {
			continue
		}

		score := freeMemory

		if score > maxScore {
			maxScore = score
			bestNode = &nodes[i]
		}
	}

	return bestNode
}
