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

The controller uses the current Kubernetes client configuration to connect to the cluster.

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

If the maximum retry count is exceeded, the error is logged and the key is forgotten. Processing can resume if a subsequent event enqueues the key again.

The controller does not currently implement a periodic resynchronization mechanism to recover keys after the retry limit is exceeded.

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
| Worker | 5 tests covering successful reconciliation, retry, recovery, retry exhaustion and shutdown |
| Informer | 8 event-handler subtests plus 1 initial-event/shutdown test |

The tests use fake Kubernetes clients and in-memory Workqueues to verify behavior without requiring a running Kubernetes cluster.

Worker and Informer tests have also passed repeated Race Detector runs.

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

## Current Limitations

- Controls one Deployment per process.
- Watches Deployment Add events and changes to `spec.replicas`; other updates are not reconciled automatically.
- Does not enqueue Deployment Delete events.
- Does not perform periodic resynchronization.
- Stops retrying a failed queue item after five retries unless a new event re-enqueues it.
- Does not yet run as a Kubernetes Deployment.
- Automated tests use fake Kubernetes clients; broader integration and end-to-end tests against a real API server have not been implemented.
- The Informer shutdown test currently uses a short timing delay rather than explicit cache-sync completion signaling.

## Future Improvements

- Deploy the controller as a Kubernetes Deployment with appropriate RBAC permissions.
- Add automated integration tests against a Kubernetes API server.
- Improve cache-sync synchronization in the Informer tests.
- Consider periodic resynchronization or alternative recovery behavior after retry exhaustion.
- Support additional Deployment events and multiple managed resources.
- Remove the unused legacy `POLL_INTERVAL` configuration.
