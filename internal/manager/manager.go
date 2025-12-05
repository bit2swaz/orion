package manager

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/bit2swaz/orion/internal/cluster"
	"github.com/bit2swaz/orion/internal/scheduler"
	"github.com/bit2swaz/orion/internal/store"
	"github.com/bit2swaz/orion/internal/task"
	"github.com/bit2swaz/orion/internal/worker"
)

type Manager struct {
	Store     *store.Store
	Scheduler *scheduler.Scheduler
	Worker    *worker.Worker
	Cluster   *cluster.Cluster
	LocalID   string
}

func New(store *store.Store, scheduler *scheduler.Scheduler, worker *worker.Worker, cluster *cluster.Cluster, localID string) *Manager {
	return &Manager{
		Store:     store,
		Scheduler: scheduler,
		Worker:    worker,
		Cluster:   cluster,
		LocalID:   localID,
	}
}

func (m *Manager) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Reconcile()
		}
	}
}

func (m *Manager) Reconcile() {
	tasks, err := m.Store.ListTasks()
	if err != nil {
		log.Printf("Error listing tasks: %v", err)
		return
	}

	for _, t := range tasks {
		if t.State == task.Scheduled && t.NodeID == m.LocalID {
			m.execTask(t)
		}
	}

	if m.Store.IsLeader() {
		m.scheduleTasks(tasks)
	}
}

func (m *Manager) execTask(t *task.Task) {
	ctx := context.Background()
	_, err := m.Worker.Run(ctx, *t)
	if err != nil {
		if strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "already in use") {
			log.Printf("Task %s already running", t.ID)
			t.State = task.Running
		} else {
			log.Printf("Error running task %s: %v", t.ID, err)
			t.State = task.Failed
		}
	} else {
		t.State = task.Running
	}

	if m.Store.IsLeader() {
		event := task.TaskEvent{
			ID:        t.ID,
			State:     t.State,
			Timestamp: time.Now(),
			Task:      *t,
		}

		data, err := json.Marshal(event)
		if err != nil {
			log.Printf("Error marshaling event: %v", err)
			return
		}

		if future := m.Store.R.Apply(data, 10*time.Second); future.Error() != nil {
			log.Printf("Error applying to Raft: %v", future.Error())
		}
	} else {
		log.Printf("Node %s is not leader, cannot update task %s state to %v", m.LocalID, t.ID, t.State)
	}
}

func (m *Manager) scheduleTasks(tasks []*task.Task) {
	for _, t := range tasks {
		if t.State == task.Pending {
			members := m.Cluster.Members()
			var nodes []scheduler.Node
			for _, member := range members {
				nodes = append(nodes, scheduler.Node{
					ID:          member.Name,
					MemoryTotal: 4 * 1024 * 1024 * 1024,
					MemoryUsed:  0,
					DiskTotal:   100 * 1024 * 1024 * 1024,
					DiskUsed:    0,
					Tags:        map[string]string{},
				})
			}

			candidate := m.Scheduler.SelectCandidate(*t, nodes)
			if candidate != nil {
				t.NodeID = candidate.ID
				t.State = task.Scheduled

				event := task.TaskEvent{
					ID:        t.ID,
					State:     task.Scheduled,
					Timestamp: time.Now(),
					Task:      *t,
				}

				data, err := json.Marshal(event)
				if err != nil {
					log.Printf("Error marshaling event: %v", err)
					continue
				}

				if future := m.Store.R.Apply(data, 10*time.Second); future.Error() != nil {
					log.Printf("Error applying to Raft: %v", future.Error())
				}
			}
		}
	}
}
