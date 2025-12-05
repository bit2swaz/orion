package worker

import (
	"context"
	"testing"

	"github.com/bit2swaz/orion/internal/task"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

func TestWorker_Run(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatalf("Failed to create docker client: %v", err)
	}

	w := &Worker{
		Name:   "test-worker",
		Client: cli,
	}

	task := task.Task{
		ID:      uuid.New(),
		Name:    "test-container-" + uuid.New().String(),
		Image:   "alpine",
		Command: []string{"echo", "hello"},
		Memory:  128 * 1024 * 1024,
		Cpu:     0.5,
	}

	dockerID, err := w.Run(context.Background(), task)

	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}
	if dockerID == "" {
		t.Fatal("Run() returned empty DockerID")
	}
	t.Logf("Container started with ID: %s", dockerID)

	err = w.Stop(dockerID)
	if err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}
