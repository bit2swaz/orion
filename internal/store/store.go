package store

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bit2swaz/orion/internal/task"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type Store struct {
	R  *raft.Raft
	db map[string]*task.Task
	mu sync.RWMutex
}

func New() *Store {
	return &Store{
		db: make(map[string]*task.Task),
	}
}

func (s *Store) Apply(l *raft.Log) interface{} {
	var event task.TaskEvent
	if err := json.Unmarshal(l.Data, &event); err != nil {
		panic(fmt.Sprintf("failed to unmarshal command: %s", err.Error()))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch event.State {
	case task.Pending:
		s.db[event.Task.ID.String()] = &event.Task
	case task.Completed, task.Failed:
		if t, ok := s.db[event.Task.ID.String()]; ok {
			t.State = event.State
			t.FinishTime = event.Timestamp
		} else {
			s.db[event.Task.ID.String()] = &event.Task
		}
	default:
		s.db[event.Task.ID.String()] = &event.Task
	}

	return nil
}

func (s *Store) Snapshot() (raft.FSMSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	o := make(map[string]*task.Task)
	for k, v := range s.db {
		o[k] = v
	}
	return &fsmSnapshot{store: o}, nil
}

func (s *Store) Restore(rc io.ReadCloser) error {
	o := make(map[string]*task.Task)
	if err := json.NewDecoder(rc).Decode(&o); err != nil {
		return err
	}

	s.db = o
	return nil
}

func (s *Store) GetTask(id string) (*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.db[id]
	if !ok {
		return nil, fmt.Errorf("task not found")
	}
	return t, nil
}

func (s *Store) ListTasks() ([]*task.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tasks []*task.Task
	for _, t := range s.db {
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Store) IsLeader() bool {
	return s.R.State() == raft.Leader
}

type fsmSnapshot struct {
	store map[string]*task.Task
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, err := json.Marshal(f.store)
		if err != nil {
			return err
		}

		if _, err := sink.Write(b); err != nil {
			return err
		}

		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *fsmSnapshot) Release() {}

func (s *Store) Open(dataDir string, localID string, bindAddr string, bootstrap bool) error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localID)

	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	snapshots, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stderr)
	if err != nil {
		return err
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore

	boltDB, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("new bolt store: %s", err)
	}
	logStore = boltDB
	stableStore = boltDB

	ra, err := raft.NewRaft(config, s, logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.R = ra

	if bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		s.R.BootstrapCluster(configuration)
	}

	return nil
}
