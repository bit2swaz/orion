package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/bit2swaz/orion/internal/cluster"
	"github.com/bit2swaz/orion/internal/manager"
	"github.com/bit2swaz/orion/internal/scheduler"
	"github.com/bit2swaz/orion/internal/store"
	"github.com/bit2swaz/orion/internal/task"
	"github.com/bit2swaz/orion/internal/worker"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	apiPort    int
	gossipPort int
	raftPort   int
	nodeID     string
	joinAddr   string
	bootstrap  bool
)

var rootCmd = &cobra.Command{
	Use:   "orion",
	Short: "Orion is a distributed task scheduler",
	Run: func(cmd *cobra.Command, args []string) {
		hostname, _ := os.Hostname()
		if nodeID == "" {
			nodeID = fmt.Sprintf("%s-%d", hostname, gossipPort)
		}

		fmt.Printf("Starting Raft on port %d (Bootstrap: %v)\n", raftPort, bootstrap)
		dataDir := fmt.Sprintf("data-%s", nodeID)
		os.MkdirAll(dataDir, 0755)

		s := store.New()
		raftAddr := fmt.Sprintf("127.0.0.1:%d", raftPort)
		if err := s.Open(dataDir, nodeID, raftAddr, bootstrap); err != nil {
			fmt.Printf("Failed to open Raft store: %v\n", err)
			os.Exit(1)
		}

		conf := cluster.GetLifeguardConfig()
		conf.BindPort = gossipPort
		conf.Name = nodeID

		c, err := cluster.New(conf)
		if err != nil {
			fmt.Printf("Failed to create cluster: %v\n", err)
			os.Exit(1)
		}

		w, err := worker.New(nodeID)
		if err != nil {
			fmt.Printf("Failed to create worker: %v\n", err)
			os.Exit(1)
		}

		sched := scheduler.New()
		mgr := manager.New(s, sched, w, c, nodeID)
		go mgr.Run(context.Background())

		if joinAddr != "" {
			_, err := c.Join([]string{joinAddr})
			if err != nil {
				fmt.Printf("Failed to join cluster: %v\n", err)
			} else {
				fmt.Printf("Joined cluster at %s\n", joinAddr)
			}
		}

		http.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
			members := c.Members()
			var nodes []map[string]interface{}
			for _, m := range members {
				node := map[string]interface{}{
					"name":   m.Name,
					"ip":     m.Addr.String(),
					"role":   "worker",
					"status": "alive",
				}
				nodes = append(nodes, node)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(nodes)
		})

		http.HandleFunc("/raft", func(w http.ResponseWriter, r *http.Request) {
			state := "Follower"
			if s.IsLeader() {
				state = "Leader"
			}
			tasks, _ := s.ListTasks()
			resp := map[string]interface{}{
				"state":    state,
				"taskCont": len(tasks),
				"tasks":    tasks,
			}
			json.NewEncoder(w).Encode(resp)
		})

		http.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}

			var t task.Task
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			t.ID = uuid.New()
			t.State = task.Pending
			t.StartTime = time.Now()

			event := task.TaskEvent{
				ID:        t.ID,
				State:     task.Pending,
				Timestamp: time.Now(),
				Task:      t,
			}

			data, err := json.Marshal(event)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if future := s.R.Apply(data, 10*time.Second); future.Error() != nil {
				http.Error(w, future.Error().Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(t)
		})

		fmt.Printf("Starting API server on port %d\n", apiPort)
		fmt.Printf("Gossip listening on port %d\n", gossipPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", apiPort), nil); err != nil {
			fmt.Printf("Error starting API server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.Flags().IntVar(&apiPort, "port", 8080, "API server port")
	rootCmd.Flags().IntVar(&gossipPort, "gossip-port", 7946, "Gossip port")
	rootCmd.Flags().IntVar(&raftPort, "raft-port", 7000, "Raft port")
	rootCmd.Flags().StringVar(&nodeID, "id", "", "Node ID")
	rootCmd.Flags().StringVar(&joinAddr, "join", "", "Address of peer to join")
	rootCmd.Flags().BoolVar(&bootstrap, "bootstrap", false, "Bootstrap the Raft cluster")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
