
# Go Kubernetes Controller PoC

A minimal event-driven Kubernetes controller implemented in Go using `client-go`.

This project demonstrates the fundamental architecture of a Kubernetes controller, including SharedInformer, Workqueue, reconciliation, retry handling, and graceful shutdown.

The controller watches a single Kubernetes Deployment and automatically restores its desired replica count when it is changed.

## Project Goals

- Understand Kubernetes controller architecture through implementation.
- Replace periodic polling with event-driven reconciliation.
- Use SharedInformer and typed Workqueue to process Deployment events.
- Implement retry handling with exponential backoff.
- Verify controller behavior using fake Kubernetes clients and unit tests.
- Demonstrate practical Go and Kubernetes development skills.

This project is intended for learning and technical demonstration, not production use.

## Getting Started

### Prerequisites

- Go
- Docker
- kind
- kubectl
- A working local Kubernetes cluster

### Build

```bash
go mod download
go build -o bin/controller ./cmd/controller
```

### Prepare a Kubernetes Deployment

Create a namespace:

```bash
kubectl create namespace go-k8s-poc
```

Create a sample Deployment:

```bash
kubectl create deployment sample-app \
  --image=nginx \
  -n go-k8s-poc
```

If the namespace or Deployment already exists, skip its creation.

### Run the Controller

Make sure your current Kubernetes context points to the intended cluster.

```bash
kubectl config current-context
```

Start the controller:

```bash
NAMESPACE=go-k8s-poc \
DEPLOYMENT_NAME=sample-app \
DESIRED_REPLICAS=3 \
./bin/controller
```

When running locally, the controller uses KUBECONFIG or ~/.kube/config. Inside Kubernetes, it uses InClusterConfig and the Pod's ServiceAccount.

### Verify Automatic Reconciliation

In another terminal, change the Deployment's replica count:

```bash
kubectl scale deployment sample-app \
  -n go-k8s-poc \
  --replicas=1
```

The controller should detect the change and restore the desired replica count to 3.

```bash
kubectl get deployment sample-app -n go-k8s-poc
```

Stop the controller with `Ctrl+C`.

## Project Structure

```text
cmd/
  controller/
    main.go

internal/
  config/
    config.go
    config_test.go

  controller/
    controller.go
    controller_test.go

  informer/
    deployment.go
    deployment_test.go

  kubernetes/
    client.go
    scale.go

  worker/
    worker.go
    worker_test.go
```

Responsibilities:

- `cmd/controller`: Application startup, dependency wiring, and shutdown.
- `internal/config`: Environment-variable configuration and validation.
- `internal/controller`: Reconciliation logic.
- `internal/informer`: Deployment event monitoring and queue registration.
- `internal/kubernetes`: Kubernetes client creation and scale operations.
- `internal/worker`: Queue processing, retries, and error handling.

## Architecture

The controller uses an event-driven reconciliation architecture built with Kubernetes `client-go`.

```text
Kubernetes API Server
        |
        v
SharedInformer
        |
        v
Rate-Limiting Workqueue
        |
        v
Worker
        |
        v
ReconcileDeployment
        |
        v
GetScale / UpdateScale
        |
        v
Kubernetes Deployment
```

The SharedInformer watches a specific Deployment and enqueues its namespace/name key when an Add event occurs or the desired replica count changes.

The Worker processes queued keys and compares the Deployment's current desired replica count with the configured target.

If the values differ, the controller updates the Deployment's scale subresource. If they match, no update is performed.

## Configuration

The controller supports configuration through environment variables.

| Variable | Default | Description |
|---|---|---|
| `NAMESPACE` | `go-k8s-poc` | Target Kubernetes namespace |
| `DEPLOYMENT_NAME` | `sample-app` | Target Deployment |
| `DESIRED_REPLICAS` | `3` | Desired replica count (0 or greater) |
| `POLL_INTERVAL` | `5s` | Legacy configuration; currently unused in event-driven mode |

Example:

```bash
NAMESPACE=go-k8s-poc \
DEPLOYMENT_NAME=sample-app \
DESIRED_REPLICAS=3 \
./bin/controller
```

When running locally, the controller uses KUBECONFIG or ~/.kube/config. Inside Kubernetes, it uses InClusterConfig and the Pod's ServiceAccount.

## Event-Driven Reconciliation

The controller uses a SharedInformer instead of periodic polling.

Events are processed as follows:

1. The Informer detects the initial Deployment or a change to its desired replica count.
2. The Deployment key is added to a typed Workqueue.
3. The Worker retrieves the key and invokes `ReconcileDeployment`.
4. The controller compares the current and configured replica counts.
5. If necessary, the controller calls `UpdateScale`.
6. A successful reconciliation clears the key's retry history.

The Workqueue also deduplicates keys, helping avoid redundant concurrent processing of the same Deployment.

## Retry and Error Handling

The controller uses a typed rate-limiting Workqueue with exponential backoff.

| Setting | Value |
|---|---|
| Initial retry delay | 1 second |
| Maximum retry delay | 30 seconds |
| Maximum retries | 5 |
| Maximum attempts per queued item | 6 (initial attempt + 5 retries) |

If reconciliation fails, the Worker schedules a retry with exponential backoff.

After successful reconciliation, retry history is cleared.

### Recovery After Maximum Retries

The worker supports automatic recovery from prolonged Kubernetes API failures.

- Failed reconciliation attempts use exponential backoff.
- After five rate-limited retries, the retry history is reset.
- The Deployment key is scheduled again after a 60-second recovery delay.
- If the API remains unavailable, another retry cycle begins.
- Once the API recovers, reconciliation resumes normally.

The recovery delay is configurable internally for unit testing.
Production execution uses a fixed 60-second delay.

Delayed queue entries are maintained in memory and are not persisted
across controller restarts.

Unit tests cover:
- Transient API failures
- Retry history reset
- Recovery scheduling after maximum retries
- Persistent API failures across retry cycles
- Successful reconciliation after API recovery
- Worker shutdown

## Graceful Shutdown

The controller handles SIGINT and SIGTERM using `signal.NotifyContext()`.

When a shutdown signal is received:

1. The context is cancelled.
2. The Informer stops watching for events.
3. The Workqueue is shut down.
4. The Worker exits.
5. The controller logs its shutdown completion.

This allows the controller to terminate without leaving its Worker running.

## Testing

Run all unit tests:

```bash
go test ./... -v -count=1
```

Run Worker and Informer tests with the Race Detector:

```bash
go test -race ./internal/worker -count=10
go test -race ./internal/informer -count=10
```

Run static analysis:

```bash
go vet ./...
```

Build:

```bash
go build -o bin/controller ./cmd/controller
```

The test suite covers:

| Component | Coverage |
|---|---|
| Configuration | 11 subtests covering environment-variable parsing and validation |
| Reconciliation | 6 subtests covering scale comparison, API interactions and error handling |
| Worker | 6 tests covering successful reconciliation, retry, recovery, retry exhaustion and shutdown |
| Informer | 8 event-handler subtests plus 1 initial-event/shutdown test |

The tests use fake Kubernetes clients and in-memory Workqueues to verify behavior without requiring a running Kubernetes cluster.

Worker and Informer tests have also passed repeated Race Detector runs.

The Informer tests use an explicit readiness channel to verify
successful initial cache synchronization before triggering shutdown.
This avoids relying on fixed sleep durations.

## Manual Verification

The controller has been manually verified using a local Kubernetes cluster created with kind.

Example: change the target Deployment's replica count.

```bash
kubectl scale deployment sample-app \
  -n go-k8s-poc \
  --replicas=1
```

When the controller is running with `DESIRED_REPLICAS=3`, it detects the change and restores the desired replica count to 3.

Verify the result:

```bash
kubectl get deployment sample-app -n go-k8s-poc
```

The expected desired replica count is 3. Pod readiness may take additional time.

## Containerized Deployment (Phase 4)

The controller can run as a Kubernetes Deployment in a kind cluster.

### Container Image

The Dockerfile uses a multi-stage build:

- Build stage: Go 1.26
- Runtime stage: Distroless static Debian 12 (nonroot)
- CGO disabled
- Stripped Go binary
- No shell or package manager in the runtime image

Build the image:

```bash
docker build -t go-k8s-controller:v0.1.0 .
```

The Dockerfile and `.dockerignore` are included in this repository.

### Kubernetes Authentication

The Kubernetes client supports two execution environments:

| Environment | Configuration |
|---|---|
| Local execution | `KUBECONFIG` or `~/.kube/config` |
| Kubernetes Pod | `rest.InClusterConfig()` |

The Pod authenticates using its ServiceAccount.

### RBAC

`deploy/rbac.yaml` defines:

- ServiceAccount: `go-k8s-controller`
- Role: `go-k8s-controller`
- RoleBinding: `go-k8s-controller`

The Role is scoped to the `go-k8s-poc` namespace.

| Resource | Allowed verbs |
|---|---|
| `deployments.apps` | `get`, `list`, `watch` |
| `deployments.apps/scale` | `get`, `update` |

Deployment deletion is not granted by this Role.

### Deploy to kind

Make sure the target context is correct:

```bash
kubectl config current-context
```

For the verified environment, the context is `kind-go-k8s-poc`.

Prepare the namespace and sample Deployment using the Getting Started instructions if they do not already exist.

Apply the RBAC configuration:

```bash
kubectl apply -f deploy/rbac.yaml
```

Load the locally built image into kind:

```bash
kind load docker-image go-k8s-controller:v0.1.0 --name go-k8s-poc
```

Deploy the controller:

```bash
kubectl apply -f deploy/controller.yaml
```

Wait for rollout:

```bash
kubectl rollout status deployment/go-k8s-controller   -n go-k8s-poc --timeout=120s
```

Check Pod status and logs:

```bash
kubectl get pods -n go-k8s-poc

kubectl logs -n go-k8s-poc   deployment/go-k8s-controller --tail=100
```

The Deployment uses one controller replica and targets `sample-app` with `DESIRED_REPLICAS=3`.

The Pod security settings include:

- Non-root UID/GID 65532
- `runAsNonRoot: true`
- RuntimeDefault seccomp profile
- Read-only root filesystem
- Privilege escalation disabled
- Linux capabilities dropped

### In-Cluster Manual Verification

On October 14, 2026, the controller was deployed successfully to the `go-k8s-poc` kind cluster.

The controller Pod reached `1/1 Running`, and the rollout completed successfully.

The following command intentionally changed the target Deployment to one replica:

```bash
kubectl scale deployment/sample-app   -n go-k8s-poc --replicas=1
```

Observed controller logs:

```text
[Informer] enqueued go-k8s-poc/sample-app
[Worker] reconciling go-k8s-poc/sample-app attempt=1
Reconciling deployment go-k8s-poc/sample-app: current=1 desired=3
Scaling deployment go-k8s-poc/sample-app: 1 -> 3
[Informer] enqueued go-k8s-poc/sample-app
[Worker] reconciling go-k8s-poc/sample-app attempt=1
Reconciling deployment go-k8s-poc/sample-app: current=3 desired=3
No scaling required
```

Final observed state:

```text
NAME         READY   UP-TO-DATE   AVAILABLE
sample-app   3/3     3            3
```

This verifies event detection, queue processing, reconciliation, scale updates, and no-op behavior after the desired state is restored.

### Troubleshooting Note

During deployment, the initial Pod failed with `CreateContainerConfigError`.

The kubelet reported that it could not verify the image's named `nonroot` user against `runAsNonRoot: true`.

Explicitly setting `runAsUser: 65532` and `runAsGroup: 65532` in the Pod security context resolved the startup issue.

## Current Limitations

- Controls one Deployment per process.
- Watches Deployment Add events and changes to `spec.replicas`; other updates are not reconciled automatically.
- Does not enqueue Deployment Delete events.
- Does not perform periodic resynchronization.
- After five rate-limited retries, schedules another reconciliation after 60 seconds; delayed queue entries are not persisted across restarts.
- Runs as a Kubernetes Deployment, but automated in-cluster failure recovery tests are not yet implemented.
- Automated tests use fake Kubernetes clients; broader integration and end-to-end tests against a real API server have not been implemented.

## Future Improvements

- Add automated deployment and RBAC verification.
- Add automated integration tests against a Kubernetes API server.
- Consider periodic resynchronization in addition to the existing delayed retry mechanism.
- Support additional Deployment events and multiple managed resources.
- Remove the unused legacy `POLL_INTERVAL` configuration.
