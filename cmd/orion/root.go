package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/bit2swaz/orion/internal/cluster"
	"github.com/spf13/cobra"
)

var (
	apiPort    int
	gossipPort int
	joinAddr   string
)

var rootCmd = &cobra.Command{
	Use:   "orion",
	Short: "Orion is a distributed task scheduler",
	Run: func(cmd *cobra.Command, args []string) {
		conf := cluster.GetLifeguardConfig()
		conf.BindPort = gossipPort

		hostname, _ := os.Hostname()
		conf.Name = fmt.Sprintf("%s-%d", hostname, gossipPort)

		c, err := cluster.New(conf)
		if err != nil {
			fmt.Printf("Failed to create cluster: %v\n", err)
			os.Exit(1)
		}

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
					"cpu":    0.0,     
					"ram":    0,
				}
				nodes = append(nodes, node)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(nodes)
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
	rootCmd.Flags().StringVar(&joinAddr, "join", "", "Address of peer to join")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
