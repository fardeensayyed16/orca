# Orca — Lightweight Container Orchestrator

![Orca Launch Video](docs/media/orca-launch.gif)

A lightweight, multi-host container orchestrator that manages Docker containers across multiple machines on a LAN.


> *"Orca decides. Orclet executes."*

---

## 🎯 Overview

Orca is a multi-host container orchestration platform that schedules, runs, and manages Docker containers across worker nodes. It provides scheduling, health checking, auto-scaling, high availability, load balancing, and a web dashboard — all in ~5000 lines of Go.

*Two binaries:*
- *orca* — Control plane daemon + CLI + GUI (runs on one machine)
- *orclet* — Worker agent (runs on every machine that runs containers)

## ✨ Features

- 🎯 *Resource-Aware Scheduling* — Most-Free-CPU-First placement with deterministic tie-breaking
- 🔄 *Reverse Proxy with Round-Robin* — Service traffic load balanced across healthy replicas
- 🛡️ *HA Reconciliation* — Automatic reschedule of orphaned replicas when a node fails
- 📈 *Horizontal Pod Autoscaler (HPA)* — CPU-based auto-scaling with configurable min/max/target
- 💚 *Health Probes* — Liveness and readiness checks (HTTP/TCP/exec)
- 🔁 *Restart Policies* — Always, OnFailure, Never with exponential backoff
- 🖥️ *Web GUI Dashboard* — Browser-based monitoring with auto-refresh (htmx)
- 🔧 *Full CLI* — deploy, get, scale, delete, status, token show
- 💾 *SQLite Persistence* — Cluster state survives daemon restarts
- 🔐 *Bearer Token Auth* — Simple shared-token authentication for LAN deployments

---

## 🚀 Quick Start

## 🖥️ Platform Support

Orca is a lightweight, multi-host container orchestration platform designed to manage container workloads across multiple worker machines.

Development and testing are performed on Windows using WSL2 with Ubuntu and Docker Desktop.

### 🐧 Recommended: Windows with WSL2

WSL2 provides a Linux environment on Windows and works with Docker Desktop's WSL2 backend. This setup is recommended for development and testing.

*Prerequisites:*
- Windows 10/11 with WSL2 enabled.
- Ubuntu installed in WSL2.
- Docker Desktop with the WSL2 backend enabled.
- Go 1.22 or later.
- Git.

*Install Go in WSL2 Ubuntu:*

Download a compatible Go release from the official Go downloads page and follow the Linux installation instructions.

Verify the installation:

```bash
go version

### Prerequisites

- *Go 1.22+*
- *Docker* in Linux containers mode (Docker Desktop with WSL2 backend or native Linux)
- *2+ machines* on the same LAN (or localhost for single-machine testing)

### 1. Clone and Build

```bash
git clone https://github.com/fardeensayyed16/orca.git
git clone https://github.com/fardeensayyed16/orclet.git

cd orca && go build -o orca ./cmd/orca
cd ../orclet && go build -o orclet ./cmd/orclet

##Build and run Orca:
cd ~/orca-project/orca

go build -o orca ./cmd/orca

./orca daemon \
  --bind-address=0.0.0.0:9001 \
  --advertise-address=<LAN-IP>

  ##Windows Firewall Configuration
Run PowerShell as Administrator on the daemon machine if remote worker machines need access.
Allow the Orca API port:netsh advfirewall firewall add rule name="Orca-API" dir=in action=allow protocol=TCP localport=9001
If your deployment uses the service reverse-proxy port range, configure the corresponding firewall rule according to your network and security requirements.
Restrict inbound access to trusted worker machines whenever possible.
🔌 Network Ports
Port / Range
Purpose
9001
Orca HTTP API
9000
GUI, if configured on this port
30000–32767
Service reverse-proxy pool, if configured
Actual ports depend on the application's configuration.



## ⚠️ Scope and Limitations

This is an *educational project* targeting a trusted LAN environment.

- *No TLS* — Bearer token travels in plaintext. Treat LAN as trusted.
- *Single control plane* — No leader election or failover for the daemon.
- *Linux containers only* — Docker must be in Linux containers mode.
- *Host port pool* — Limited to ~1000 published ports per node (40000-41000).
- *No RBAC* — Single shared token for CLI and orclets.

---

## 📄 License

[MIT License](LICENSE)