package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fardeensayyed16/orca/pkg/types"
	"github.com/spf13/cobra"
)

var (
	deployImage            string
	deployReplicas         int
	deployPort             int
	deployTargetPort       int
	deployEnv              []string
	deployCommand          string
	deployCPU              string
	deployMemory           string
	deployRestartPolicy    string
	deployPullPolicy       string
	deployLivenessHTTPPath string
	deployLivenessHTTPPort int
	deployReadinessTCPPort int
	deployInitialDelay     int
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service-name> [flags]",
	Short: "Deploy a service specification to Orca cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		if deployImage == "" {
			return fmt.Errorf("--image is required")
		}
		if deployReplicas < 1 {
			return fmt.Errorf("--replicas must be greater than zero")
		}
		containerPort := deployPort
		if deployTargetPort > 0 {
			if containerPort > 0 && containerPort != deployTargetPort {
				return fmt.Errorf("--port and --target-port must match")
			}
			containerPort = deployTargetPort
		}
		cpuMillis, err := parseCPU(deployCPU)
		if err != nil {
			return err
		}
		memoryBytes, err := parseMemory(deployMemory)
		if err != nil {
			return err
		}
		command := strings.Fields(deployCommand)
		env := make(map[string]string, len(deployEnv))
		for _, entry := range deployEnv {
			key, value, ok := strings.Cut(entry, "=")
			if !ok || key == "" {
				return fmt.Errorf("invalid --env %q: expected KEY=VALUE", entry)
			}
			env[key] = value
		}
		liveness, err := createProbe("liveness", containerPort, deployLivenessHTTPPath, deployLivenessHTTPPort, 0, deployInitialDelay)
		if err != nil {
			return err
		}
		readiness, err := createProbe("readiness", containerPort, "", 0, deployReadinessTCPPort, deployInitialDelay)
		if err != nil {
			return err
		}
		payload := map[string]any{
			"name": args[0], "image": deployImage, "replicas": deployReplicas,
			"cpu_millis": cpuMillis, "memory_bytes": memoryBytes,
			"container_port": containerPort, "restart_policy": deployRestartPolicy,
			"pull_policy": deployPullPolicy, "env": env, "command": command,
		}
		if liveness != nil {
			payload["liveness"] = liveness
		}
		if readiness != nil {
			payload["readiness"] = readiness
		}
		body, status, err := apiRequest(cmd, "POST", "/api/v1/services", payload)
		if err != nil {
			return err
		}
		if status != 201 {
			return responseError(status, body)
		}
		fmt.Printf("Service %s deployed\n", args[0])
		fmt.Print(string(body))
		return nil
	},
}

func parseCPU(value string) (int64, error) {
	if value == "" {
		return 1000, nil
	}
	if strings.HasSuffix(value, "m") {
		millis, err := strconv.ParseInt(strings.TrimSuffix(value, "m"), 10, 64)
		if err != nil || millis <= 0 {
			return 0, fmt.Errorf("invalid --cpu %q: expected a positive millicore value", value)
		}
		return millis, nil
	}
	cores, err := strconv.ParseFloat(value, 64)
	if err != nil || cores <= 0 {
		return 0, fmt.Errorf("invalid --cpu %q: expected cores or millicores", value)
	}
	millis := int64(cores * 1000)
	if millis <= 0 {
		return 0, fmt.Errorf("invalid --cpu %q: value is less than 1m", value)
	}
	return millis, nil
}

func parseMemory(value string) (int64, error) {
	if value == "" {
		return 512 * 1024 * 1024, nil
	}
	units := []struct {
		suffix string
		mult   int64
	}{
		{"Gi", 1024 * 1024 * 1024}, {"G", 1000 * 1000 * 1000},
		{"Mi", 1024 * 1024}, {"M", 1000 * 1000},
		{"Ki", 1024}, {"K", 1000},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			number, err := strconv.ParseInt(strings.TrimSuffix(value, unit.suffix), 10, 64)
			if err != nil || number <= 0 {
				break
			}
			return number * unit.mult, nil
		}
	}
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil || bytes <= 0 {
		return 0, fmt.Errorf("invalid --memory %q: expected bytes or a Ki/Mi/Gi value", value)
	}
	return bytes, nil
}

func createProbe(kind string, containerPort int, httpPath string, httpPort, tcpPort, initialDelay int) (*types.Probe, error) {
	if httpPort < 0 || tcpPort < 0 {
		return nil, fmt.Errorf("%s probe port cannot be negative", kind)
	}
	if initialDelay < 0 {
		return nil, fmt.Errorf("--initial-delay cannot be negative")
	}
	probes := 0
	if httpPort > 0 || httpPath != "" {
		probes++
	}
	if tcpPort > 0 {
		probes++
	}
	if probes > 1 {
		return nil, fmt.Errorf("specify only one %s probe type", kind)
	}
	if probes == 0 {
		return nil, nil
	}
	if containerPort == 0 && (httpPort > 0 || httpPath != "" || tcpPort > 0) {
		if httpPort > 0 || httpPath != "" {
			return nil, fmt.Errorf("--port is required when --%s-http-port is set", kind)
		}
		return nil, fmt.Errorf("--port is required when --%s-tcp-port is set", kind)
	}
	probe := &types.Probe{Kind: kind, InitialDelaySeconds: initialDelay}
	switch {
	case httpPort > 0 || httpPath != "":
		if httpPort > 0 && httpPort != containerPort {
			return nil, fmt.Errorf("--%s-http-port must match --port (%d)", kind, containerPort)
		}
		probe.Type, probe.Port, probe.Path = "http", containerPort, httpPath
		if probe.Path == "" {
			probe.Path = "/"
		}
	case tcpPort > 0:
		if tcpPort != containerPort {
			return nil, fmt.Errorf("--%s-tcp-port must match --port (%d)", kind, containerPort)
		}
		probe.Type, probe.Port = "tcp", containerPort
	}
	return probe, nil
}

func init() {
	deployCmd.Flags().StringVar(&deployImage, "image", "", "Container image")
	deployCmd.Flags().IntVar(&deployReplicas, "replicas", 1, "Number of replicas")
	deployCmd.Flags().IntVar(&deployPort, "port", 0, "Container port to publish through the service proxy")
	deployCmd.Flags().IntVar(&deployTargetPort, "target-port", 0, "Container target port")
	deployCmd.Flags().StringSliceVar(&deployEnv, "env", nil, "Environment variables (KEY=VALUE)")
	deployCmd.Flags().StringVar(&deployCommand, "command", "", "Container entrypoint/command override")
	deployCmd.Flags().StringVar(&deployCPU, "cpu", "", "CPU limit (e.g. 500m, 1)")
	deployCmd.Flags().StringVar(&deployMemory, "memory", "", "Memory limit (e.g. 256Mi)")
	deployCmd.Flags().StringVar(&deployRestartPolicy, "restart", "Always", "Restart policy (Always, OnFailure, Never)")
	deployCmd.Flags().StringVar(&deployRestartPolicy, "restart-policy", "Always", "Alias for --restart")
	deployCmd.Flags().StringVar(&deployPullPolicy, "pull-policy", "IfNotPresent", "Pull policy (IfNotPresent, Always, Never)")

	// Supported v1 probe flags
	deployCmd.Flags().StringVar(&deployLivenessHTTPPath, "liveness-http-path", "", "Liveness HTTP probe path")
	deployCmd.Flags().IntVar(&deployLivenessHTTPPort, "liveness-http-port", 0, "Liveness HTTP probe port")
	deployCmd.Flags().IntVar(&deployReadinessTCPPort, "readiness-tcp-port", 0, "Readiness TCP probe port")
	deployCmd.Flags().IntVar(&deployInitialDelay, "initial-delay", 0, "Probe initial delay in seconds")

	deployCmd.Args = cobra.ExactArgs(1)
	rootCmd.AddCommand(deployCmd)
}
