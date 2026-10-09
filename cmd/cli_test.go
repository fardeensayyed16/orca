package cmd

import (
	"testing"
)

func TestParseCPUAndMemory(t *testing.T) {
	cpu, err := parseCPU("1500m")
	if err != nil || cpu != 1500 {
		t.Fatalf("parseCPU = %d, %v; want 1500, nil", cpu, err)
	}
	cpu, err = parseCPU("1.5")
	if err != nil || cpu != 1500 {
		t.Fatalf("parseCPU cores = %d, %v; want 1500, nil", cpu, err)
	}
	if _, err := parseCPU("-1"); err == nil {
		t.Fatal("negative CPU unexpectedly accepted")
	}
	memory, err := parseMemory("512Mi")
	if err != nil || memory != 512*1024*1024 {
		t.Fatalf("parseMemory = %d, %v; want 512Mi", memory, err)
	}
	if _, err := parseMemory("invalid"); err == nil {
		t.Fatal("invalid memory unexpectedly accepted")
	}
}

func TestCreateProbeRequiresPublishedPortAndMatchesContainerPort(t *testing.T) {
	if _, err := createProbe("liveness", 0, "/health", 8080, 0, 0); err == nil {
		t.Fatal("HTTP probe without --port unexpectedly accepted")
	}
	probe, err := createProbe("readiness", 8080, "", 0, 8080, 0)
	if err != nil {
		t.Fatal(err)
	}
	if probe == nil || probe.Type != "tcp" || probe.Port != 8080 || probe.Kind != "readiness" {
		t.Fatalf("TCP probe = %+v", probe)
	}
	if _, err := createProbe("readiness", 8080, "", 0, 8081, 0); err == nil {
		t.Fatal("probe port that differs from --port unexpectedly accepted")
	}
	if _, err := createProbe("readiness", 0, "", 0, 8080, 0); err == nil {
		t.Fatal("TCP probe without --port unexpectedly accepted")
	}
}

func TestLoopbackServerValidation(t *testing.T) {
	for _, server := range []string{"http://127.0.0.1:9001", "http://localhost:9001", "http://[::1]:9001"} {
		if err := requireLoopbackServer(server); err != nil {
			t.Errorf("loopback server %q rejected: %v", server, err)
		}
	}
	for _, server := range []string{"https://orca.example:9001", "http://192.0.2.1:9001"} {
		if err := requireLoopbackServer(server); err == nil {
			t.Errorf("non-loopback server %q accepted", server)
		}
	}
}

func TestCommandTreeIncludesPhaseEightResources(t *testing.T) {
	for _, path := range [][]string{
		{"deploy"}, {"get", "services"}, {"get", "service"}, {"get", "nodes"},
		{"get", "node"}, {"get", "pods"}, {"get", "pod"}, {"scale"},
		{"delete", "service"}, {"delete", "node"}, {"status"}, {"token", "show"}, {"gui"},
	} {
		command, _, err := rootCmd.Find(path)
		if err != nil || command == nil {
			t.Errorf("command %v not found: %v", path, err)
			continue
		}
	}
}
