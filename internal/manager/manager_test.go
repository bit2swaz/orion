package manager

import (
	"encoding/json"
	"testing"

	"github.com/bit2swaz/orion/internal/cluster"
	"github.com/hashicorp/memberlist"
)

type MockCluster struct{}

func (m *MockCluster) Members() []*memberlist.Node {
	meta := cluster.NodeMeta{
		ID:          "worker-1",
		Role:        "worker",
		MemoryTotal: 8 * 1024 * 1024 * 1024,
		MemoryUsed:  1 * 1024 * 1024 * 1024,
	}
	metaBytes, _ := json.Marshal(meta)

	return []*memberlist.Node{
		{
			Name: "worker-1",
			Meta: metaBytes,
		},
	}
}

func TestManager_Struct_Compile(t *testing.T) {
	_ = &Manager{
		Cluster: &cluster.Manager{},
	}
}
