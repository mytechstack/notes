.PHONY: build run mock dev dev-split bench test test-integration lint clean \
        build-hydration build-reader docker-build docker-up docker-down

BIN         := bin/server
MOCKBIN     := bin/mockbackend
BENCHBIN    := bin/benchmark
HYDBIN      := bin/hydration-server
READERBIN   := bin/context-reader

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	go build -o $(BIN) ./cmd/server

build-mock:
	go build -o $(MOCKBIN) ./cmd/mockbackend

build-bench:
	go build -o $(BENCHBIN) ./cmd/benchmark

build-hydration:
	go build -o $(HYDBIN) ./cmd/hydration-server

build-reader:
	go build -o $(READERBIN) ./cmd/context-reader

# ── Run ───────────────────────────────────────────────────────────────────────

# Combined server (all routes on :8080) + mock backend — for local development
# Usage: make dev
dev: build build-mock
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@./$(MOCKBIN) -port=9000 & MOCK_PID=$$!; \
	trap "kill $$MOCK_PID 2>/dev/null" EXIT INT TERM; \
	export $$(grep -v '^#' .env | xargs) && ./$(BIN)

# Split production services: hydration (:8080) + context-reader (:8081) + mock
# Usage: make dev-split
dev-split: build-hydration build-reader build-mock
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@./$(MOCKBIN) -port=9000 & MOCK_PID=$$!; \
	./$(READERBIN) & READER_PID=$$!; \
	trap "kill $$MOCK_PID $$READER_PID 2>/dev/null" EXIT INT TERM; \
	export $$(grep -v '^#' .env | xargs) && ./$(HYDBIN)

# Server only (no mock — point service URLs at real backends)
run: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs) && ./$(BIN); else ./$(BIN); fi

# Run only the mock backend
# Usage: make mock  OR  make mock PORT=9000 LATENCY=100ms
mock: build-mock
	./$(MOCKBIN) -port=$(or $(PORT),9000) -latency=$(or $(LATENCY),50ms)

# ── Benchmark ─────────────────────────────────────────────────────────────────

# Run benchmark against already-running dev stack (make dev in another terminal)
# Override: make bench N=200 RESOURCE=permissions LATENCY=100ms
bench: build-bench
	./$(BENCHBIN) \
		-hydrator=http://localhost:8080 \
		-backend=http://localhost:9000 \
		-n=$(or $(N),100) \
		-resource=$(or $(RESOURCE),profile)

# ── Test ──────────────────────────────────────────────────────────────────────

test:
	go test ./... -count=1

test-integration:
	go test ./... -count=1 -tags integration -run Integration

lint:
	go vet ./...

# ── Docker ────────────────────────────────────────────────────────────────────

docker-build:
	docker build -t context-hydrator .

docker-up:
	docker compose up --build

docker-down:
	docker compose down

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/
