package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/bit2swaz/orion/internal/store"
)

func TestCluster_Join(t *testing.T) {
	dirA := t.TempDir()
	storeA := store.New()
	raftPortA := 18000
	raftAddrA := fmt.Sprintf("127.0.0.1:%d", raftPortA)
	if err := storeA.Open(dirA, "NodeA", raftAddrA, true); err != nil {
		t.Fatalf("Failed to open Store A: %v", err)
	}
	time.Sleep(2 * time.Second)

	gossipPortA := 17946
	nodeA, err := New(gossipPortA, raftPortA, "NodeA", "manager", storeA)
	if err != nil {
		t.Fatalf("Failed to create Node A: %v", err)
	}
	defer nodeA.Leave()

	dirB := t.TempDir()
	storeB := store.New()
	raftPortB := 18001
	raftAddrB := fmt.Sprintf("127.0.0.1:%d", raftPortB)
	if err := storeB.Open(dirB, "NodeB", raftAddrB, false); err != nil {
		t.Fatalf("Failed to open Store B: %v", err)
	}

	gossipPortB := 18946
	nodeB, err := New(gossipPortB, raftPortB, "NodeB", "worker", storeB)
	if err != nil {
		t.Fatalf("Failed to create Node B: %v", err)
	}
	defer nodeB.Leave()

	joinAddr := fmt.Sprintf("127.0.0.1:%d", gossipPortB)
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

	future := storeA.R.GetConfiguration()
	if err := future.Error(); err != nil {
		t.Fatalf("Failed to get Raft configuration: %v", err)
	}
}
