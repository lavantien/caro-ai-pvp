BINARY := bin/caro$(shell CGO_ENABLED=1 go env GOEXE)
LOGSTATS_BIN := bin/logstats$(shell CGO_ENABLED=1 go env GOEXE)
COVERPROFILE := coverage.out

MERMAID_CLI_VERSION := 12.0.0
PUPPETEER_VERSION := 25.12.0
DIAGRAMS_DIR := docs/diagrams
DIAGRAM_BG := rgb(13,17,23)
DIAGRAM_SRC := $(wildcard $(DIAGRAMS_DIR)/*.mmd)
DIAGRAM_PNG := $(DIAGRAM_SRC:.mmd=.png)

.PHONY: all doctor fmt fmt-check lint vet build test test-race cover bench fuzz mutate mutate-resume run serve migrate tourney-smoke-32 tourney-smoke-10 tourney-full firewall tidy diagrams diagrams-browser ci

all: build

doctor:
	@set -e; \
	echo "go: $$(CGO_ENABLED=1 go version)"; \
	echo "cc: $$($$(command -v gcc || command -v clang) --version | head -1)"; \
	echo "make: $$(make --version | head -1)"; \
	v=$$(CGO_ENABLED=1 go env GOVERSION | sed 's/^go//'); \
	major=$${v%%.*}; rest=$${v#*.}; minor=$${rest%%.*}; patch=0; \
	case $$rest in *.*) patch=$${rest##*.};; esac; \
	score=$$((major*1000000 + minor*1000 + patch)); \
	if [ $$score -lt 1027001 ]; then echo "doctor: need go >= 1.27.1, got $$v"; exit 1; fi; \
	if [ "$$(CGO_ENABLED=1 go env CGO_ENABLED)" != "1" ]; then echo "doctor: CGO_ENABLED=1 not accepted"; exit 1; fi; \
	command -v gcc >/dev/null 2>&1 || command -v clang >/dev/null 2>&1 || { echo "doctor: no C toolchain (gcc or clang) in PATH"; exit 1; }; \
	echo "doctor: ok"

fmt:
	@gofmt -w $$(git ls-files '*.go')

fmt-check:
	@out=$$(gofmt -l $$(git ls-files '*.go')); \
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
		CGO_ENABLED=1 go test -run='^$$' -fuzz=FuzzRulesDifferential -fuzztime=60s ./internal/rules; \
	else echo "fuzz: no targets yet, internal/rules lands at M1"; fi
	@if [ -d internal/server ]; then \
		for t in FuzzRatingLaw FuzzSeriesDrive FuzzMovesBlob; do \
			CGO_ENABLED=1 go test -run='^$$' -fuzz=$$t -fuzztime=30s ./internal/server || exit 1; \
		done; \
	fi
	@if [ -d internal/tourney ]; then \
		for t in FuzzPairings FuzzFoldZeroSum; do \
			CGO_ENABLED=1 go test -run='^$$' -fuzz=$$t -fuzztime=30s ./internal/tourney || exit 1; \
		done; \
	fi

# mutate [PARALLEL=1] [CHALLENGE=1]: mutation gate over the core packages.
# CHALLENGE=1 runs the suite under allowlisted mutants too, auditing every
# equivalence proof (milestone-closing runs should set it).
mutate:
	CGO_ENABLED=1 go run ./cmd/mutate -allow .mutate-allow -parallel $(or $(PARALLEL),1) $(if $(CHALLENGE),-challenge)

# mutate-resume LOG=prior-run.log continues a gate after a host failure:
# prior KILLED verdicts are replayed, everything else is re-decided fresh.
mutate-resume:
	CGO_ENABLED=1 go run ./cmd/mutate -allow .mutate-allow -parallel $(or $(PARALLEL),1) $(if $(CHALLENGE),-challenge) -resume "$(LOG)"

run:
	CGO_ENABLED=1 go run ./cmd/caro $(or $(ARGS),ports)

migrate:
	CGO_ENABLED=1 go run ./cmd/caro migrate

serve:
	CGO_ENABLED=1 go run ./cmd/caro serve

# Headless tournament drivers of first-cause.md Scenario 2. Each prints the
# per-series lines and the final leaderboard to stdout and writes the
# per-series txt logs and the summary into their own timestamped folder under
# logs/tourny/ (config.TournamentLogRoot). ARGS
# forwards flags, e.g. make tourney-smoke-10 ARGS="--db scratch.db --parallel 1".
tourney-smoke-32:
	CGO_ENABLED=1 go run ./cmd/caro tourney smoke32 $(ARGS)

tourney-smoke-10:
	CGO_ENABLED=1 go run ./cmd/caro tourney smoke10 $(ARGS)

tourney-full:
	CGO_ENABLED=1 go run ./cmd/caro tourney full $(ARGS)

firewall:
	CGO_ENABLED=1 go run ./cmd/caro firewall

tidy:
	CGO_ENABLED=1 go mod tidy

# diagrams renders every docs/diagrams/*.mmd to a dark-mode PNG next to its
# source (docs/diagrams/<name>.png, embedded by README.md) at 3x scale for
# zoom legibility. The theme and colors live in mermaid-config.json, the
# browser flags in puppeteer.json; editing either re-renders everything.
# Idempotent: only inputs newer than their PNG re-render. Needs node >= 22.13
# and network on the first run (npx fetches mermaid-cli plus its puppeteer
# peer, and diagrams-browser installs the headless browser into the user
# cache at %USERPROFILE%/.cache/puppeteer).
diagrams: diagrams-browser $(DIAGRAM_PNG)

# mermaid-cli 12 takes puppeteer as a peer dependency and npx never runs
# package install scripts, so the browser download has to be explicit. The
# version is pinned by the puppeteer package, matching $(PUPPETEER_VERSION).
diagrams-browser:
	npx -y puppeteer@$(PUPPETEER_VERSION) browsers install chrome-headless-shell

$(DIAGRAMS_DIR)/%.png: $(DIAGRAMS_DIR)/%.mmd $(DIAGRAMS_DIR)/mermaid-config.json $(DIAGRAMS_DIR)/puppeteer.json
	npx -y -p @mermaid-js/mermaid-cli@$(MERMAID_CLI_VERSION) -p puppeteer@$(PUPPETEER_VERSION) mmdc -i $< -o $@ -e png -b '$(DIAGRAM_BG)' -s 3 -c $(DIAGRAMS_DIR)/mermaid-config.json -p $(DIAGRAMS_DIR)/puppeteer.json

ci: fmt-check lint vet build test-race cover
	@echo "ci: all green"

# logstats renders the markdown evidence report of the v0.20 chain from the
# per-series txt tournament logs: inventory, per-participant standings
# folded through the runner's seat law, per-tier M-line telemetry, a
# strength verdict with tier-inversion lines, and a parse-integrity ledger
# listing every line the analyzer rejected. The analyzer lives in its own
# nested module (playground/logstats, replace-pinned to the root module the
# same way playground/series is), and a nested go.mod is invisible to the
# parent module's package patterns, so it builds with -C at its own
# directory and runs as a bin/ binary instead of go run: go -C would also
# re-point the tool's working directory, breaking the relative --dir and
# --out paths ARGS carries. Empty ARGS reads the newest run folder under
# config.TournamentLogRoot (logs/tourny), e.g.
# make logstats ARGS="--dir logs/tourny/<run> --out report.md".
.PHONY: logstats
logstats:
	CGO_ENABLED=1 go -C playground/logstats build -o ../../$(LOGSTATS_BIN) .
	$(LOGSTATS_BIN) $(ARGS)
