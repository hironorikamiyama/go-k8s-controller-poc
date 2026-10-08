
## Configuration

The controller supports configuration through environment variables.

| Variable | Default | Description |
|---|---|---|
| `NAMESPACE` | `go-k8s-poc` | Target Kubernetes namespace |
| `DEPLOYMENT_NAME` | `sample-app` | Target Deployment |
| `DESIRED_REPLICAS` | `3` | Desired replica count (0 or greater) |
| `POLL_INTERVAL` | `5s` | Reconciliation interval |

Example:

```bash
NAMESPACE=go-k8s-poc \
DEPLOYMENT_NAME=sample-app \
DESIRED_REPLICAS=3 \
POLL_INTERVAL=2s \
./bin/controller
```

## Graceful Shutdown

The controller handles SIGINT and SIGTERM using
`signal.NotifyContext()`.

When a shutdown signal is received, the context is
cancelled and the polling loop terminates.

## Testing

Run unit tests:

```bash
go test ./... -v -count=1
```

Run static analysis:

```bash
go vet ./...
```

Build:

```bash
go build -o bin/controller ./cmd/controller
```

The current test suite contains 17 table-driven subtests:

- 6 reconciliation tests
- 11 configuration tests

The reconciliation tests use a fake Kubernetes client to
verify API call counts, desired replica values,
resourceVersion propagation, and error handling.

## Current Limitations

- Uses polling rather than Informer/Watch.
- Controls one Deployment per process.
- Does not implement automatic conflict retries.
- Does not yet run as a Kubernetes Deployment.
- Integration tests against a real API server are limited.
