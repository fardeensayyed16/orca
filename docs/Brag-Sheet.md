# 🐋 Orca & Orclet: Engineering Brag Document & Impact Report

> *"Orca decides. Orclet executes."*
> **A multi-host, distributed container orchestrator written from scratch in ~8,800 lines of Go.**

---

## Executive Summary

**Orca** is a production-grade, distributed container orchestration system built from the ground up in Go. Inspired by Kubernetes and Nomad, it decouples cluster orchestration into a centralized control plane (`orca`) and lightweight node agents (`orclet`). It implements distributed state persistence, deterministic scheduling, autonomous container lifecycle management, reverse-proxy load balancing, horizontal auto-scaling (HPA), and self-healing high availability (HA).

```
                      +-----------------------------------+
                      |      Operator (CLI / Web GUI)     |
                      +-----------------------------------+
                                        |  HTTP REST / Bearer Auth
                                        v
                       +----------------------------------+
                       |           ORCA DAEMON            |
                       |  - REST API Engine & Auth        |
                       |  - Resource-Aware Scheduler      |
                       |  - Dynamic Reverse Proxy (L4/L7) |
                       |  - Self-Healing HA Reconciler    |
                       |  - Horizontal Pod Autoscaler     |
                       |  - SQLite Storage Engine (ACID)  |
                       +----------------------------------+
                                  |            |
                 Heartbeat / Work |            | Heartbeat / Work
                       (5s Poll)  |            | (5s Poll)
                                  v            v
            +---------------------------+   +---------------------------+
            |      ORCLET WORKER 1      |   |      ORCLET WORKER 2      |
            | - Docker Moby Engine SDK  |   | - Docker Moby Engine SDK  |
            | - 3-Axis Health Engine    |   | - 3-Axis Health Engine    |
            | - Container Adoption      |   | - Container Adoption      |
            | - Port Pool (40000-41000) |   | - Port Pool (40000-41000) |
            | - Real-time Stats Sampler |   | - Real-time Stats Sampler |
            +---------------------------+   +---------------------------+
```

---

## 🏆 Key Numbers & Engineering Metrics

| Metric | Measurement | Impact / Context |
| :--- | :--- | :--- |
| **Total Lines of Code** | **~8,800 lines of Go** | Clean, modular codebase split across `orca`, `orclet`, and stdlib `pkg/types` |
| **System Components** | **2 distinct binaries** | Control plane daemon (`orca`) + node worker agent (`orclet`) |
| **Architectural Phases** | **10 distinct phases** | Built incrementally from raw daemon to HPA and HTMX GUI |
| **End-to-End Test Plan** | **34 comprehensive test cases** | Full lifecycle validation covering edge cases, failovers, and crashes |
| **HA Recovery Window** | **< 90 seconds** | Automatic detection of dead workers and transparent rescheduling of orphaned workloads |
| **Failover Heartbeat** | **5s interval / 15s threshold** | Fast detection of unreachability without thrashing or false positives |
| **Dependency Footprint** | **Zero heavyweight deps** | Pure Go embedded SQLite (`modernc.org/sqlite`), standard library HTTP/proxy, official Docker Moby SDK |

---

## 🛠️ Deep Dive: The Hard Engineering Problems Solved

### 1. The 3-Axis Container Lifecycle Model
Most naive container managers conflate container run state with application health. Orca disentangles lifecycle into **three orthogonal axes**:
* **State** (`Starting`, `Running`, `Exited`): What Docker reports about the process.
* **Health** (`Healthy`, `Unhealthy`): Whether internal liveness probes pass.
* **Ready** (`Ready`, `Not-Ready`): Whether readiness probes pass, determining if traffic should be routed to this replica.

> **Why this matters:** Allows zero-downtime rollouts, warming up caches before taking traffic, and separating transient startup lags from broken application code.

### 2. Zero-Loss Container Adoption & Idempotent Restarts
When an `orclet` worker crashes or restarts:
* It does **not** blindly wipe running containers.
* It interrogates Docker via labels (`orca.service`, `orca.replica-id`, `orca.restart-count`, `orca.host-ip`).
* Compares running containers against the desired state pulled from the `/api/v1/work` endpoint.
* Reconnects to running workloads instantly, defeating recreate-window races by taking `max(observed_label, desired)` for restart tracking.
* Automatically detects node IP changes and safely repaves containers only when host binding mismatches occur.

### 3. High Availability (HA) & Autonomous Reconciler
* **Self-Healing Loop:** Continuously evaluates node heartbeats. If a node fails to report within 15 seconds, it is marked `NotReady`.
* **Orphan Rescheduling:** The HA engine unbinds orphaned replicas and reschedules them to healthy nodes within ~65–90 seconds.
* **Graceful Draining:** Safely drains nodes undergoing maintenance, honoring graceful eviction before node deletion.
* **Priority Victim Selection:** On scale-down operations, replicas are terminated in strict deterministic order:
  $$\text{Pending} \longrightarrow \text{Not-Ready} \longrightarrow \text{Newest Ready}$$

### 4. Dynamic Reverse Proxy with Service Discovery
* Orca implements an internal L4/L7 reverse proxy that automatically binds dedicated service ports (`30000–32767`).
* Every service is accessible at `http://<daemon-ip>:<service-port>` regardless of which worker node hosts the actual container.
* Dynamic round-robin load balancer continuously filters backends, only forwarding traffic to replicas that are **both** `Running` and `Ready`.
* Dynamically allocates and releases host ports from each worker's pool (`40000–41000`).

### 5. Horizontal Pod Autoscaler (HPA)
* Orclet samples container CPU and memory usage every 15s via Docker's streaming stats API.
* Orca aggregates metrics across all active replicas of a service.
* Calculates desired replicas dynamically:
  $$\text{Desired Replicas} = \left\lceil \text{Current Replicas} \times \frac{\text{Average CPU \%}}{\text{Target CPU \%}} \right\rceil$$
* Incorporates hysteresis and scale-down cooldown windows to prevent rapid flapping under bursty workloads.

### 6. Security & Isolation by Design
* **Bearer Token Authentication:** Simple, unified token validation protecting all mutating and scheduling API endpoints.
* **Loopback Admin Protection:** Web GUI (`127.0.0.1:9000`) and administrative token endpoints enforce strict HTTP `Host` and `Origin` header loopback checks, denying external access with HTTP 403.
* **Rule-E Address Conflict Prevention:** Enforces strict node address consistency to prevent split-brain registrations.

---

## 💼 Resume & Portfolio Bullets

Here are ready-to-use bullet points tailored for software engineering resumes, LinkedIn, and performance reviews:

### High-Impact / Staff-Level Bullets
* **Architected and implemented Orca**, a lightweight distributed container orchestrator (~8,800 LoC in Go) featuring resource-aware scheduling, dynamic reverse proxying, HPA auto-scaling, and autonomous self-healing.
* **Engineered a resilient agent-worker architecture (`orclet`)** utilizing the Docker Moby SDK, featuring zero-downtime container adoption across agent reboots and an orthogonal 3-axis lifecycle engine (State, Health, Ready).
* **Built an autonomous HA reconciler** capable of detecting worker node partitions (15s threshold) and transparently rescheduling orphaned workloads within 90s, achieving automated fault tolerance without cluster degradation.
* **Designed a dynamic reverse proxy and port management subsystem**, automatically provisioning host port pools (40000–41000) and service ingress listeners (30000–32767) with real-time round-robin load balancing across healthy replicas.
* **Implemented an embedded, zero-dependency Web UI & CLI** using Go templates, HTMX, and SQLite ACID persistence, protected by loopback origin validation and token-based API authentication.

---

## 🎤 Interview "Tell Me About a Time" Stories

### Story 1: Solving the "Recreate-Window Race" on Agent Restarts
* **Situation:** When worker nodes restart, running containers must not be terminated or duplicated, and restart counts must remain strictly synchronized between the control plane and worker.
* **Task:** Build an idempotent container adoption mechanism that bridges Docker state and control plane desired state.
* **Action:** Embedded metadata directly into Docker container labels at runtime (`orca.service`, `orca.replica-id`, `orca.restart-count`). On boot, `orclet` reads labels and executes a reconciliation algorithm using `max(observed_label, desired)` to defeat out-of-order race conditions.
* **Result:** Zero downtime across worker agent restarts, seamless re-attachment, and deterministic state recovery.

### Story 2: Graceful Scale-Down with Priority Victim Selection
* **Situation:** Scaling down a multi-replica service naively can kill active user-facing requests or break high-priority jobs.
* **Task:** Determine the optimal, non-disruptive order for killing replica instances.
* **Action:** Implemented a multi-stage victim selector algorithm that prioritizes terminating replicas in `Pending` state first, followed by unhealthy/warming-up (`Not-Ready`) replicas, and only terminating `Ready` instances starting with the newest (least traffic-warm).
* **Result:** Minimized dropped connections and guaranteed maximum availability during aggressive auto-scale downs.

---

## 🧭 Tech Stack Breakdown

* **Language:** Go (1.22+)
* **Container Runtime:** Docker Engine, Moby SDK (`github.com/moby/moby/client`), `containerd/errdefs`
* **Storage:** SQLite (`modernc.org/sqlite` embedded pure Go), in-memory MapStorage for unit tests
* **Networking:** Go `net/http`, `httputil.ReverseProxy`, TCP port allocators
* **Frontend:** Go `html/template`, HTMX (reactive live polling), embedded assets (`embed.FS`)
* **Platforms:** Linux & Windows (with native `sc.exe` Windows Service lifecycle integration)
