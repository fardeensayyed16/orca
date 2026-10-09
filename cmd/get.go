package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fardeensayyed16/orca/pkg/types"
	"github.com/spf13/cobra"
)

type serviceListItem struct {
	types.Service
	ReplicaSummary struct {
		Desired int `json:"desired"`
		Running int `json:"running"`
	} `json:"replica_summary"`
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get resources from the Orca cluster",
	RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
}

var getServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var services []serviceListItem
		if err := decodeResponse(cmd, "/api/v1/services", &services); err != nil {
			return err
		}
		var replicas []types.Replica
		if err := decodeResponse(cmd, "/api/v1/pods", &replicas); err != nil {
			return err
		}
		ready := make(map[string]int)
		restarts := make(map[string]int)
		for _, replica := range replicas {
			if replica.Status == types.PodStatusRunning && replica.Health == types.HealthStatusHealthy && replica.Ready {
				ready[replica.ServiceName]++
			}
			restarts[replica.ServiceName] += replica.RestartCount
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "NAME\tREPLICAS\tREADY\tPORT\tRESTART")
		for _, service := range services {
			port := "-"
			if service.ServicePort > 0 {
				port = fmt.Sprint(service.ServicePort)
			}
			fmt.Fprintf(writer, "%s\t%d\t%d\t%s\t%d\n",
				service.Name, service.ReplicaSummary.Desired, ready[service.Name], port, restarts[service.Name])
		}
		return writer.Flush()
	},
}

var getServiceCmd = &cobra.Command{
	Use:   "service <name>",
	Short: "Show a service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var services []serviceListItem
		if err := decodeResponse(cmd, "/api/v1/services", &services); err != nil {
			return err
		}
		for _, service := range services {
			if service.Name == args[0] {
				fmt.Printf("NAME: %s\nIMAGE: %s\nREPLICAS: %d\nCONTAINER PORT: %d\nSERVICE PORT: %d\nRESTART: %s\n",
					service.Name, service.Image, service.Replicas, service.ContainerPort, service.ServicePort, service.RestartPolicy)
				return nil
			}
		}
		return fmt.Errorf("service %q not found", args[0])
	},
}

var getNodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "List worker nodes",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var nodes []types.Node
		if err := decodeResponse(cmd, "/api/v1/nodes", &nodes); err != nil {
			return err
		}
		var services []serviceListItem
		if err := decodeResponse(cmd, "/api/v1/services", &services); err != nil {
			return err
		}
		serviceResources := make(map[string]types.Service, len(services))
		for _, service := range services {
			serviceResources[service.Name] = service.Service
		}
		var replicas []types.Replica
		if err := decodeResponse(cmd, "/api/v1/pods", &replicas); err != nil {
			return err
		}
		usedCPU := make(map[string]int64)
		usedMemory := make(map[string]int64)
		for _, replica := range replicas {
			if replica.Status != types.PodStatusRunning && replica.Status != types.PodStatusStarting {
				continue
			}
			service := serviceResources[replica.ServiceName]
			usedCPU[replica.NodeName] += service.CPUMillis
			usedMemory[replica.NodeName] += service.MemoryBytes
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "NAME\tSTATUS\tCPU (USED/TOTAL)\tMEM (USED/TOTAL)\tLAST SEEN\tADDRESS")
		for _, node := range nodes {
			fmt.Fprintf(writer, "%s\t%s\t%s/%s\t%s/%s\t%s\t%s\n",
				node.Name, node.Status,
				formatCPU(usedCPU[node.Name]), formatCPU(node.CapacityCPUMillis),
				formatBytes(usedMemory[node.Name]), formatBytes(node.CapacityMemoryBytes),
				node.LastSeen.Format("2006-01-02 15:04:05"), node.Address)
		}
		return writer.Flush()
	},
}

var getNodeCmd = &cobra.Command{
	Use:   "node <name>",
	Short: "Show a worker node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var node types.Node
		if err := decodeResponse(cmd, "/api/v1/nodes/"+escapedPathSegment(args[0]), &node); err != nil {
			return err
		}
		fmt.Printf("NAME: %s\nSTATUS: %s\nADDRESS: %s\nCPU: %s\nMEMORY: %s\nLAST SEEN: %s\n",
			node.Name, node.Status, node.Address, formatCPU(node.CapacityCPUMillis),
			formatBytes(node.CapacityMemoryBytes), node.LastSeen.Format("2006-01-02 15:04:05"))
		return nil
	},
}

var getPodsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List replicas",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var replicas []types.Replica
		if err := decodeResponse(cmd, "/api/v1/pods", &replicas); err != nil {
			return err
		}
		writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(writer, "SERVICE\tNODE\tCONTAINER ID\tSTATUS\tHEALTH\tREADY\tRESTARTS")
		for _, replica := range replicas {
			ready := "false"
			if replica.Ready {
				ready = "true"
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n",
				replica.ServiceName, replica.NodeName, replica.ContainerID,
				replica.Status, replica.Health, ready, replica.RestartCount)
		}
		return writer.Flush()
	},
}

var getPodCmd = &cobra.Command{
	Use:   "pod <id>",
	Short: "Show a replica",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var replicas []types.Replica
		if err := decodeResponse(cmd, "/api/v1/pods", &replicas); err != nil {
			return err
		}
		for _, replica := range replicas {
			if replica.ID == args[0] {
				fmt.Printf("ID: %s\nSERVICE: %s\nNODE: %s\nCONTAINER ID: %s\nSTATUS: %s\nHEALTH: %s\nREADY: %t\nRESTARTS: %d\n",
					replica.ID, replica.ServiceName, replica.NodeName, replica.ContainerID,
					replica.Status, replica.Health, replica.Ready, replica.RestartCount)
				return nil
			}
		}
		return fmt.Errorf("pod %q not found", args[0])
	},
}

func formatCPU(millis int64) string {
	return fmt.Sprintf("%dm", millis)
}

func formatBytes(value int64) string {
	if value >= 1024*1024*1024 && value%(1024*1024*1024) == 0 {
		return fmt.Sprintf("%dGi", value/(1024*1024*1024))
	}
	if value >= 1024*1024 && value%(1024*1024) == 0 {
		return fmt.Sprintf("%dMi", value/(1024*1024))
	}
	if value >= 1024 && value%1024 == 0 {
		return fmt.Sprintf("%dKi", value/1024)
	}
	return fmt.Sprintf("%dB", value)
}

func init() {
	getCmd.AddCommand(getServicesCmd, getServiceCmd, getNodesCmd, getNodeCmd, getPodsCmd, getPodCmd)
	rootCmd.AddCommand(getCmd)
}
