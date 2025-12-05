package cluster

import (
	"fmt"
	"io"
	"testing"
	"time"
)

func TestCluster_Join(t *testing.T) {
	confA := GetLifeguardConfig()
	confA.Name = "NodeA"
	confA.BindAddr = "127.0.0.1"
	confA.BindPort = 7946
	confA.LogOutput = io.Discard

	confB := GetLifeguardConfig()
	confB.Name = "NodeB"
	confB.BindAddr = "127.0.0.1"
	confB.BindPort = 8946
	confB.LogOutput = io.Discard

	nodeA, err := New(confA)
	if err != nil {
		t.Fatalf("Failed to create Node A: %v", err)
	}
	defer nodeA.Leave(time.Second)

	nodeB, err := New(confB)
	if err != nil {
		t.Fatalf("Failed to create Node B: %v", err)
	}
	defer nodeB.Leave(time.Second)

	joinAddr := fmt.Sprintf("%s:%d", confB.BindAddr, confB.BindPort)
	numJoined, err := nodeA.Join([]string{joinAddr})
	if err != nil {
		t.Fatalf("Node A failed to join Node B: %v", err)
	}
	if numJoined != 1 {
		t.Errorf("Expected 1 node joined, got %d", numJoined)
	}

	time.Sleep(2 * time.Second)

	membersA := nodeA.Members()
	if len(membersA) != 2 {
		t.Errorf("Node A expected 2 members, got %d", len(membersA))
	}

	membersB := nodeB.Members()
	if len(membersB) != 2 {
		t.Errorf("Node B expected 2 members, got %d", len(membersB))
	}
}
