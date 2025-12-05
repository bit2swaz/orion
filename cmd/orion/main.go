package main

import (
	"context"
	"fmt"

	"github.com/bit2swaz/orion/internal/task"
	"github.com/bit2swaz/orion/internal/worker"
	"github.com/google/uuid"
)

func main() {
	w, _ := worker.New("test-worker")
	t := task.Task{ID: uuid.New(), Name: "test", Image: "nginx", PortBindings: map[string]string{"80/tcp": "8080"}}
	id, err := w.Run(context.Background(), t)
	fmt.Println("Container ID:", id, "Error:", err)
}
