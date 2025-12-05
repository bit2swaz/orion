package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type Node struct {
	Name   string  `json:"name"`
	IP     string  `json:"ip"`
	Role   string  `json:"role"`
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	RAM    int64   `json:"ram"`
}

var membersPort int

var membersCmd = &cobra.Command{
	Use:   "members",
	Short: "List cluster members",
	Run: func(cmd *cobra.Command, args []string) {
		url := fmt.Sprintf("http://localhost:%d/nodes", membersPort)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error connecting to API: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Error: API returned status %s\n", resp.Status)
			return
		}

		var nodes []Node
		if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
			fmt.Printf("Error decoding response: %v\n", err)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "Name\tIP\tRole\tStatus\tCPU\tRAM")
		for _, node := range nodes {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.2f\t%d\n", node.Name, node.IP, node.Role, node.Status, node.CPU, node.RAM)
		}
		w.Flush()
	},
}

func init() {
	membersCmd.Flags().IntVar(&membersPort, "port", 8080, "API server port")
	rootCmd.AddCommand(membersCmd)
}
