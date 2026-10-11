# Orca — Lightweight Container Orchestrator

![Orca Launch Video](docs/media/orca-launch.gif)

A lightweight, multi-host container orchestrator that manages Docker containers across multiple machines on a LAN.

> **"Orca decides. Orclet executes."**

---

## 🎯 Overview

Orca is a distributed container orchestration platform that schedules, runs, and manages Docker containers across worker nodes. It provides scheduling, health checking, auto-scaling, high availability, load balancing, and a web dashboard — all in ~8,800 lines of Go.

**Two binaries:**
- **`orca`** — Control plane daemon + CLI + GUI (runs on one machine)
- **`orclet`** — Worker agent (runs on every machine that runs containers)

## ✨ Features

- 🎯 **Resource-Aware Scheduling** — Most-Free-CPU-First placement with deterministic tie-breaking
- 🔄 **Reverse Proxy with Round-Robin** — Service traffic load balanced across healthy replicas
- 🛡️ **HA Reconciliation** — Automatic reschedule of orphaned replicas when a node fails
- 📈 **Horizontal Auto-Scaler (HPA)** — CPU-based auto-scaling with configurable min/max/target
- 💚 **Health Probes** — Liveness and readiness checks (HTTP/TCP/exec)
- 🔁 **Restart Policies** — Always, OnFailure, Never with exponential backoff
- 🖥️ **Web GUI Dashboard** — Browser-based monitoring with auto-refresh (htmx)
- 🔧 **Full CLI** — deploy, get, scale, delete, status, token show
- 💾 **SQLite Persistence** — Cluster state survives daemon restarts
- 🔐 **Bearer Token Auth** — Simple shared-token authentication for LAN deployments

---

## 🖥️ Platform Support

Orca is designed for **Windows-first deployment**. Development and testing were done on WSL2 Ubuntu, but the primary deployment target is **Windows machines** with Docker Desktop.

### Windows (Primary Target)

**Prerequisites:**
- Windows 10/11 with WSL2 enabled
- Docker Desktop for Windows (Linux containers mode)
- Go 1.22+ for Windows
- Git for Windows

**Setup:**

Install Go for Windows from https://go.dev/dl/ (Windows installer).
Install Git for Windows from https://git-scm.com/download/win.

Clone and build:

powershell
cd $env:USERPROFILE\Desktop
mkdir orca-project
cd orca-project

git clone https://github.com/fardeensayyed16/orca.git
git clone https://github.com/fardeensayyed16/orclet.git

cd orca
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -o orca.exe .

cd ..\orclet
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -o orclet.exe ./cmd/orclet


Run the daemon (Windows Terminal, foreground):

powershell
cd $env:USERPROFILE\Desktop\orca-project\orca
.\orca.exe daemon --bind-address=0.0.0.0:9001 --advertise-address=127.0.0.1


Note the "Generated API token" from the output.

Deploy from CLI:

powershell
.\orca.exe deploy web --image=nginx:1.27-alpine --replicas=3 --port=80 --server=http://127.0.0.1:9001 --token=<TOKEN>


Windows Firewall Rules

Run in Administrator PowerShell on the daemon machine:

powershell
netsh advfirewall firewall add rule name="orca-api" dir=in action=allow protocol=TCP localport=9001
netsh advfirewall firewall add rule name="orca-services" dir=in action=allow protocol=TCP localport=30000-32767


WSL2 / Linux (Development)

For development convenience, the same Go code builds and runs on WSL2 Ubuntu or native Linux.

bash
cd ~/orca-project/orca
go build -o orca .
./orca daemon --bind-address=0.0.0.0:9001 --advertise-address=127.0.0.1


Ports

Port Range Purpose
9001 Orca HTTP API
9000 GUI (loopback only)
30000-32767 Service reverse proxy pool
40000-41000 Orclet host port pool

---

🚀 Quick Start

Prerequisites

· Go 1.22+
· Docker Desktop in Linux containers mode
· 2+ machines on the same LAN (or localhost for single-machine testing)

1. Clone and Build

bash
git clone https://github.com/fardeensayyed16/orca.git
git clone https://github.com/fardeensayyed16/orclet.git

cd orca && go build -o orca .
cd ../orclet && go build -o orclet ./cmd/orclet


2. Start the Daemon (Machine A)

bash
cd orca
./orca daemon --bind-address=0.0.0.0:9001 --advertise-address=<DAEMON-IP>


3. Start a Worker (Machine B)

bash
cd orclet
./orclet worker --server=http://<DAEMON-IP>:9001 --token=<TOKEN> --name=node-1


4. Deploy a Service

bash
cd orca
./orca deploy web --image=nginx:1.27-alpine --replicas=3 --port=80 --server=http://<DAEMON-IP>:9001 --token=<TOKEN>


5. Access the Service

bash
./orca get services --server=http://<DAEMON-IP>:9001 --token=<TOKEN>
curl http://<DAEMON-IP>:<service_port>


6. Open the GUI Dashboard

On the daemon machine, open in browser:

http://127.0.0.1:9000

---

🛠️ Tech Stack

Layer Technology
Language Go 1.22+
Container Runtime Docker (moby/moby SDK v0.6.0)
Storage SQLite (modernc.org/sqlite, pure Go)
CLI Cobra
GUI htmx + html/template

---

📚 Documentation

· Quickstart — 10-minute setup guide
· Verification — Cross-cutting test scenarios
· Brag Sheet — Technical highlights and resume bullets
· Sign-off — v1 release checklist

---

⚠️ Scope and Limitations

Educational project targeting trusted LAN.

· No TLS — Bearer token travels in plaintext.
· Single control plane — No leader election.
· Linux containers only — Docker must be in Linux containers mode.
· Host port pool — Limited to ~1000 published ports per node (40000-41000).
· No RBAC — Single shared token for CLI and orclets.

---

📄 License

MIT License







