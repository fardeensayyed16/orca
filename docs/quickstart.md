# Orca Quickstart

## Prerequisites
- 2+ machines on the same LAN (or localhost for testing)
- Go 1.22+, Docker Desktop in Linux containers mode
- WSL2 Ubuntu or Linux

## Install
1. Clone both repos:
   ```sh
   git clone <orca-repo-url> orca
   git clone <orclet-repo-url> orclet
   ```
2. Build both:
   ```sh
   cd orca && go build -o orca ./cmd/orca
   cd ../orclet && go build -o orclet ./cmd/orclet
   ```

## Start Daemon (Machine A)
```sh
cd orca
./orca daemon --bind-address=0.0.0.0:9001 --advertise-address=<DAEMON_IP>
```
Note the "Generated API token" from output.

## Start Worker (Machine B)
```sh
cd orclet
./orclet worker --server=http://<DAEMON_IP>:9001 --token=<TOKEN> --name=node-1
```

## Deploy a Service
```sh
cd orca
./orca deploy web --image=nginx:1.27-alpine --replicas=3 --port=80 \
  --server=http://<DAEMON_IP>:9001 --token=<TOKEN>
```

## Access the Service
Browser or curl: `http://<DAEMON_IP>:<service_port>`

Find `service_port` with:
```sh
./orca get services
```

## GUI Dashboard (daemon machine only)
Browser: `http://127.0.0.1:9000`

## Auto-scaling (HPA)
```sh
./orca deploy burn --image=cpuburn:1 --replicas=1 --port=8080 \
  --autoscale --min=1 --max=5 --target-cpu=70
```

## Cleanup
```sh
./orca delete service <name>
./orca delete node <name>
```

## Troubleshooting
- "unauthorized": token mismatch. Check `./orca token show`.
- Port conflict: rebuild DB (`rm ~/.orca/orca.db*`).
- Docker socket: enable WSL integration in Docker Desktop.
