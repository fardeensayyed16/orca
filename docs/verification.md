# Cross-Cutting Verification

## 1. Orclet dies during deploy
- Setup: Start a daemon and a worker; prepare a multi-replica service.
- Steps: Stop the worker while deploying, then restart or leave it offline.
- Expected behavior: HA detects orphaned replicas and reschedules them within about 90 seconds.

## 2. Daemon crash + restart
- Setup: Start a daemon and a registered worker; deploy a service.
- Steps: Force-stop and restart the daemon using the same data directory.
- Expected behavior: SQLite preserves services; the worker re-registers and resumes work.

## 3. Container killed
- Setup: Deploy services using `Always`, `OnFailure`, and `Never` restart policies.
- Steps: Kill each service's container and observe the replicas.
- Expected behavior: `Always` restarts regardless of exit status; `OnFailure` restarts after a non-zero exit; `Never` does not restart.

## 4. Service scale-down
- Setup: Deploy a service with replicas in Pending, not-Ready, and Ready states.
- Steps: Scale the service down by multiple replicas.
- Expected behavior: Victims are selected in order: Pending, not-Ready, then newest.

## 5. Address mismatch (Rule E)
- Setup: Register an Orclet with one advertised address.
- Steps: Send a registration or heartbeat with a conflicting address.
- Expected behavior: The daemon responds with 409 Conflict and Orclet exits.

## 6. Reverse proxy round-robin
- Setup: Run at least two healthy service replicas that return distinguishable hostnames.
- Steps: Send repeated requests through the service address.
- Expected behavior: Requests are proxied across replicas, returning different hostnames.

## 7. HPA scale-up
- Setup: Deploy an autoscaled service with min 1 and a higher max.
- Steps: Turn CPU burn ON and wait for autoscaler evaluations.
- Expected behavior: Replica count increases from 1 toward max.

## 8. HPA scale-down
- Setup: Reach max replicas for an autoscaled service.
- Steps: Turn CPU burn OFF and wait through the configured scale-down window.
- Expected behavior: Replica count decreases from max toward 1 after the window.

## 9. OOM kill
- Setup: Deploy a container with a memory limit and a workload that exceeds it.
- Steps: Trigger the memory limit.
- Expected behavior: The replica message reports `OOMKilled (limit X Mi)`.

## 10. Loopback GUI security
- Setup: Start the daemon GUI on `127.0.0.1:9000`.
- Steps: Request a GUI GET route with a non-loopback `Host` header.
- Expected behavior: The request is rejected with HTTP 403.
