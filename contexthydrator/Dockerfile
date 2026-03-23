# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Cache dependency downloads separately from source changes
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/server      ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/mockbackend ./cmd/mockbackend

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/bin/server      ./server
COPY --from=builder /app/bin/mockbackend ./mockbackend

# Non-root user (distroless provides uid 65532 as "nonroot")
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["./server"]
