package worker

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/bit2swaz/orion/internal/task"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type Worker struct {
	Name      string
	Queue     chan task.Task
	Db        map[uuid.UUID]*task.Task
	TaskCount int
	Client    *client.Client
}

func (w *Worker) Run(ctx context.Context, t task.Task) (string, error) {
	reader, err := w.Client.ImagePull(ctx, t.Image, types.ImagePullOptions{})
	if err != nil {
		return "", err
	}
	io.Copy(os.Stdout, reader)

	rp := container.RestartPolicy{
		Name: container.RestartPolicyMode(t.RestartPolicy),
	}

	r := container.Resources{
		Memory:   t.Memory,
		NanoCPUs: int64(t.Cpu * 1000000000),
	}

	pb := nat.PortMap{}
	exposedPorts := map[nat.Port]struct{}{}

	for k, v := range t.PortBindings {
		newBinding := nat.PortBinding{
			HostIP:   "0.0.0.0",
			HostPort: v,
		}

		port := nat.Port(k)

		pb[port] = []nat.PortBinding{newBinding}
		exposedPorts[port] = struct{}{}
	}

	cc := container.Config{
		Image:        t.Image,
		ExposedPorts: exposedPorts,
		Cmd:          t.Command,
	}

	hc := container.HostConfig{
		RestartPolicy: rp,
		Resources:     r,
		PortBindings:  pb,
	}

	resp, err := w.Client.ContainerCreate(ctx, &cc, &hc, nil, nil, t.Name)
	if err != nil {
		return "", fmt.Errorf("error creating container: %v", err)
	}

	if err := w.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("error starting container: %v", err)
	}

	return resp.ID, nil
}

func (w *Worker) Stop(containerID string) error {
	ctx := context.Background()
	return w.Client.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (w *Worker) CollectStats() (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func New(name string) (*Worker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Worker{
		Name:      name,
		Queue:     make(chan task.Task),
		Db:        make(map[uuid.UUID]*task.Task),
		TaskCount: 0,
		Client:    cli,
	}, nil
}
