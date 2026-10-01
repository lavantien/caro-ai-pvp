BINARY := bin/caro
COVERPROFILE := coverage.out

.PHONY: all doctor fmt fmt-check lint vet build test test-race cover bench fuzz mutate run migrate firewall tidy ci

all: build

doctor:
	@set -e; \
	echo "go: $$(CGO_ENABLED=1 go version)"; \
	echo "gcc: $$(gcc -dumpversion)"; \
	echo "make: $$(make --version | head -1)"; \
	v=$$(CGO_ENABLED=1 go env GOVERSION | sed 's/^go//'); \
	major=$${v%%.*}; rest=$${v#*.}; minor=$${rest%%.*}; patch=0; \
	case $$rest in *.*) patch=$${rest##*.};; esac; \
	score=$$((major*1000000 + minor*1000 + patch)); \
	if [ $$score -lt 1027001 ]; then echo "doctor: need go >= 1.27.1, got $$v"; exit 1; fi; \
	if [ "$$(CGO_ENABLED=1 go env CGO_ENABLED)" != "1" ]; then echo "doctor: CGO_ENABLED=1 not accepted"; exit 1; fi; \
	command -v gcc >/dev/null 2>&1 || { echo "doctor: gcc not found in PATH"; exit 1; }; \
	echo "doctor: ok"

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then printf '%s\n' "$$out"; echo "fmt-check: run make fmt"; exit 1; fi

lint:
	CGO_ENABLED=1 go vet ./...

vet: lint

build:
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/caro

test:
	CGO_ENABLED=1 go test ./...

test-race:
	CGO_ENABLED=1 go test -race ./...

cover:
	CGO_ENABLED=1 go test -coverprofile=$(COVERPROFILE) -covermode=atomic ./...
	CGO_ENABLED=1 go tool cover -func=$(COVERPROFILE)
	CGO_ENABLED=1 go run ./cmd/covergate $(COVERPROFILE)

bench:
	CGO_ENABLED=1 go test -run='^$$' -bench=. -benchmem ./...

fuzz:
	@if [ -d internal/rules ]; then \
		CGO_ENABLED=1 go test -run='^$$' -fuzz=. -fuzztime=60s ./internal/rules; \
	else echo "fuzz: no targets yet, internal/rules lands at M1"; fi

mutate:
	CGO_ENABLED=1 go run ./cmd/mutate

run:
	CGO_ENABLED=1 go run ./cmd/caro $(or $(ARGS),ports)

migrate:
	@echo "migrate: not implemented until M6 (startup self-migration lands with the server)"

firewall:
	CGO_ENABLED=1 go run ./cmd/caro firewall

tidy:
	CGO_ENABLED=1 go mod tidy

ci: fmt-check lint vet build test-race cover
	@echo "ci: all green"
