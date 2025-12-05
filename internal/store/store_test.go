package store

import (
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/bit2swaz/orion/internal/task"
	"github.com/google/uuid"
	"github.com/hashicorp/raft"
)

func TestFSM(t *testing.T) {
	s := New()
	taskID := uuid.New()
	testTask := task.Task{
		ID:    taskID,
		Name:  "test-task",
		State: task.Pending,
		Image: "nginx",
	}

	event := task.TaskEvent{
		ID:        uuid.New(),
		State:     task.Pending,
		Timestamp: time.Now(),
		Task:      testTask,
	}
	data, _ := json.Marshal(event)
	log := &raft.Log{
		Data: data,
	}

	s.Apply(log)

	storedTask, err := s.GetTask(taskID.String())
	if err != nil {
		t.Fatalf("Task not found in store: %v", err)
	}
	if storedTask.State != task.Pending {
		t.Errorf("Expected state Pending, got %v", storedTask.State)
	}

	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	sink := new(mockSnapshotSink)
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("Persist failed: %v", err)
	}

	newStore := New()
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		pipeWriter.Write(sink.data)
		pipeWriter.Close()
	}()

	if err := newStore.Restore(pipeReader); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	restoredTask, err := newStore.GetTask(taskID.String())
	if err != nil {
		t.Fatalf("Task missing in restored store")
	}
	if restoredTask.Name != "test-task" {
		t.Errorf("Restored task data mismatch")
	}
}

func TestStore_Open(t *testing.T) {
	tmpDir := t.TempDir()
	s := New()

	err := s.Open(tmpDir, "node-1", "127.0.0.1:0", true)
	if err != nil {
		t.Fatalf("Failed to open store: %v", err)
	}

	time.Sleep(3 * time.Second)

	if !s.IsLeader() {
		t.Fatalf("Node did not become leader after bootstrap")
	}
}

type mockSnapshotSink struct {
	data []byte
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) {
	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *mockSnapshotSink) Close() error  { return nil }
func (m *mockSnapshotSink) ID() string    { return "mock" }
func (m *mockSnapshotSink) Cancel() error { return nil }
