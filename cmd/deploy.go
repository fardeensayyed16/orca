package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	deployImage              string
	deployReplicas           int
	deployPort               int
	deployTargetPort         int
	deployEnv                []string
	deployCommand            string
	deployCPU                string
	deployMemory             string
	deployRestartPolicy      string
	deployPullPolicy         string
	deployLivenessHTTPPath   string
	deployLivenessHTTPPort   int
	deployLivenessExec       string
	deployLivenessTCPPort    int
	deployReadinessHTTPPath  string
	deployReadinessHTTPPort  int
	deployReadinessExec      string
	deployReadinessTCPPort   int
	deployAutoscaleMin       int
	deployAutoscaleMax       int
	deployAutoscaleTargetCPU float64
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service-name> [flags]",
	Short: "Deploy a service specification to Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Deploying service to Orca...")
		return nil
	},
}

func init() {
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Container image")
	deployCmd.Flags().IntVar(&deployReplicas, "replicas", 1, "Number of replicas")
	deployCmd.Flags().IntVar(&deployPort, "port", 0, "Service published port on daemon")
	deployCmd.Flags().IntVar(&deployTargetPort, "target-port", 0, "Container target port")
	deployCmd.Flags().StringSliceVar(&deployEnv, "env", nil, "Environment variables (KEY=VALUE)")
	deployCmd.Flags().StringVar(&deployCommand, "command", "", "Container entrypoint/command override")
	deployCmd.Flags().StringVar(&deployCPU, "cpu", "", "CPU limit (e.g. 500m, 1)")
	deployCmd.Flags().StringVar(&deployMemory, "memory", "", "Memory limit (e.g. 256Mi)")
	deployCmd.Flags().StringVar(&deployRestartPolicy, "restart-policy", "Always", "Restart policy (Always, OnFailure, Never)")
	deployCmd.Flags().StringVar(&deployPullPolicy, "pull-policy", "IfNotPresent", "Pull policy (IfNotPresent, Always, Never)")

	// Liveness probe flags
	deployCmd.Flags().StringVar(&deployLivenessHTTPPath, "liveness-http-path", "", "Liveness HTTP probe path")
	deployCmd.Flags().IntVar(&deployLivenessHTTPPort, "liveness-http-port", 0, "Liveness HTTP probe port")
	deployCmd.Flags().StringVar(&deployLivenessExec, "liveness-exec", "", "Liveness exec probe command")
	deployCmd.Flags().IntVar(&deployLivenessTCPPort, "liveness-tcp-port", 0, "Liveness TCP probe port")

	// Readiness probe flags
	deployCmd.Flags().StringVar(&deployReadinessHTTPPath, "readiness-http-path", "", "Readiness HTTP probe path")
	deployCmd.Flags().IntVar(&deployReadinessHTTPPort, "readiness-http-port", 0, "Readiness HTTP probe port")
	deployCmd.Flags().StringVar(&deployReadinessExec, "readiness-exec", "", "Readiness exec probe command")
	deployCmd.Flags().IntVar(&deployReadinessTCPPort, "readiness-tcp-port", 0, "Readiness TCP probe port")

	// Autoscale flags
	deployCmd.Flags().IntVar(&deployAutoscaleMin, "autoscale-min", 0, "Autoscale minimum replicas")
	deployCmd.Flags().IntVar(&deployAutoscaleMax, "autoscale-max", 0, "Autoscale maximum replicas")
	deployCmd.Flags().Float64Var(&deployAutoscaleTargetCPU, "autoscale-target-cpu", 0, "Autoscale target CPU percentage")

	rootCmd.AddCommand(deployCmd)
}
