FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod main.go ./
# GONOSUMDB=* skips checksum DB verification (fine for a demo)
# go mod tidy resolves and pins all transitive deps, then build
RUN GONOSUMDB=* go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -o /token-manager .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /token-manager .
EXPOSE 9090 8083
CMD ["./token-manager"]
