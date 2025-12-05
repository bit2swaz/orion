package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"runtime"
	"time"

	"github.com/bit2swaz/orion/internal/store"
	"github.com/hashicorp/memberlist"
)

type NodeMeta struct {
	ID          string  `json:"id"`
	Role        string  `json:"role"`
	MemoryTotal int64   `json:"mem_total"`
	MemoryUsed  int64   `json:"mem_used"`
	CpuTotal    float64 `json:"cpu_total"`
	RaftPort    int     `json:"raft_port"`
}

type Manager struct {
	list     *memberlist.Memberlist
	store    *store.Store
	NodeID   string
	Role     string
	RaftPort int
}

func New(bindPort int, raftPort int, nodeID string, role string, s *store.Store) (*Manager, error) {
	m := &Manager{
		NodeID:   nodeID,
		Role:     role,
		RaftPort: raftPort,
		store:    s,
	}

	conf := GetLifeguardConfig()
	conf.Name = nodeID
	conf.BindPort = bindPort
	conf.Delegate = m
	conf.Events = m

	list, err := memberlist.Create(conf)
	if err != nil {
		return nil, err
	}
	m.list = list

	return m, nil
}

func GetLifeguardConfig() *memberlist.Config {
	conf := memberlist.DefaultLANConfig()
	conf.AwarenessMaxMultiplier = 8
	conf.SuspicionMult = 4
	conf.RetransmitMult = 4
	conf.IndirectChecks = 3
	conf.LogOutput = io.Discard
	return conf
}

func (m *Manager) Join(peers []string) (int, error) {
	return m.list.Join(peers)
}

func (m *Manager) Leave() error {
	return m.list.Leave(time.Second)
}

func (m *Manager) Members() []*memberlist.Node {
	return m.list.Members()
}

func (m *Manager) NodeMeta(limit int) []byte {
	meta := NodeMeta{
		ID:          m.NodeID,
		Role:        m.Role,
		MemoryTotal: 8 * 1024 * 1024 * 1024,
		MemoryUsed:  1 * 1024 * 1024 * 1024,
		CpuTotal:    float64(runtime.NumCPU()),
		RaftPort:    m.RaftPort,
	}
	b, _ := json.Marshal(meta)
	return b
}

func (m *Manager) NotifyMsg(b []byte)                         {}
func (m *Manager) GetBroadcasts(overhead, limit int) [][]byte { return nil }
func (m *Manager) LocalState(join bool) []byte                { return m.NodeMeta(0) }
func (m *Manager) MergeRemoteState(buf []byte, join bool)     {}

func (m *Manager) NotifyJoin(node *memberlist.Node) {
	if m.NodeID == node.Name {
		return
	}

	var meta NodeMeta
	if err := json.Unmarshal(node.Meta, &meta); err != nil {
		log.Printf("Failed to parse meta for %s: %v", node.Name, err)
		return
	}

	if m.store.IsLeader() {
		raftAddr := fmt.Sprintf("%s:%d", node.Addr.String(), meta.RaftPort)

		log.Printf("Gossip: Node %s joined. Adding to Raft at %s", node.Name, raftAddr)
		if err := m.store.Join(node.Name, raftAddr); err != nil {
			log.Printf("Failed to join node to Raft: %v", err)
		}
	}
}

func (m *Manager) NotifyLeave(node *memberlist.Node) {
	if m.store.IsLeader() {
		log.Printf("Gossip: Node %s left. Removing from Raft.", node.Name)
		m.store.Remove(node.Name)
	}
}

func (m *Manager) NotifyUpdate(node *memberlist.Node) {}
