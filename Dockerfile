
# Build stage
FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /controller \
    ./cmd/controller

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /controller /controller

USER nonroot:nonroot

ENTRYPOINT ["/controller"]
