.PHONY: build run mock dev bench test test-integration lint clean

BIN      := bin/server
MOCKBIN  := bin/mockbackend
BENCHBIN := bin/benchmark

build:
	go build -o $(BIN) ./cmd/server

build-mock:
	go build -o $(MOCKBIN) ./cmd/mockbackend

build-bench:
	go build -o $(BENCHBIN) ./cmd/benchmark

run: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs) && ./$(BIN); else ./$(BIN); fi

# Run only the mock backend (optionally override port/latency)
# Usage: make mock  OR  make mock PORT=9000 LATENCY=100ms
mock: build-mock
	./$(MOCKBIN) -port=$(or $(PORT),9000) -latency=$(or $(LATENCY),50ms)

# Run mock backend + main server together (requires .env pointing at localhost:9000)
dev: build build-mock
	@if [ ! -f .env ]; then cp .env.example .env; fi
	@./$(MOCKBIN) -port=9000 & MOCK_PID=$$!; \
	trap "kill $$MOCK_PID 2>/dev/null" EXIT INT TERM; \
	export $$(grep -v '^#' .env | xargs) && ./$(BIN)

# Run benchmark against already-running dev stack (make dev in another terminal)
# Override: make bench N=200 RESOURCE=permissions LATENCY=100ms
bench: build-bench
	./$(BENCHBIN) \
		-hydrator=http://localhost:8080 \
		-backend=http://localhost:9000 \
		-n=$(or $(N),100) \
		-resource=$(or $(RESOURCE),profile)

test:
	go test ./... -count=1

test-integration:
	go test ./... -count=1 -tags integration -run Integration

lint:
	go vet ./...

clean:
	rm -rf bin/
