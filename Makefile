BINARY := bin/caro$(shell CGO_ENABLED=1 go env GOEXE)
LOGSTATS_BIN := bin/logstats$(shell CGO_ENABLED=1 go env GOEXE)
COVERPROFILE := coverage.out

# The real-engine e2e in internal/tourney is wall-clock bound and a healthy
# run can exceed go test's 10m default under -race on shared CI runners, so
# every suite entry point carries one explicit ceiling: slow-but-healthy runs
# finish, a stuck engine still trips it.
GO_TEST_TIMEOUT := 25m

MERMAID_CLI_VERSION := 12.0.0
PUPPETEER_VERSION := 25.12.0
DIAGRAMS_DIR := docs/diagrams
DIAGRAM_BG := rgb(13,17,23)
DIAGRAM_SRC := $(wildcard $(DIAGRAMS_DIR)/*.mmd)
DIAGRAM_PNG := $(DIAGRAM_SRC:.mmd=.png)

.PHONY: all doctor fmt fmt-check lint vet build test test-race test-pkg cover bench fuzz mutate mutate-resume run serve migrate tourney-smoke-32 tourney-smoke-10 tourney-full tourney-resume tourney-close firewall tidy diagrams diagrams-browser darkcontrast ci

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
	CGO_ENABLED=1 go test -timeout $(GO_TEST_TIMEOUT) ./...

test-race:
	CGO_ENABLED=1 go test -race -timeout $(GO_TEST_TIMEOUT) ./...

# test-pkg PKG=./internal/tourney [ARGS="-run TestX -count=1"]: one package's
# tests under the race detector, the same mode test-race runs the tree in,
# scoped for the fix loop before a full-tree pass.
test-pkg:
	CGO_ENABLED=1 go test -race -timeout $(GO_TEST_TIMEOUT) $(ARGS) $(PKG)

cover:
	CGO_ENABLED=1 go test -coverprofile=$(COVERPROFILE) -covermode=atomic ./...
	CGO_ENABLED=1 go tool cover -func=$(COVERPROFILE)
	CGO_ENABLED=1 go run ./cmd/covergate $(COVERPROFILE)

# cover-pcts prints the badge percentages off a coverage profile: global
# over the whole profile, core as the weakest core package, plus the two
# thresholds the badge colors against (single-sourced in the config hub).
# Silent recipe: CI feeds this output straight into GITHUB_OUTPUT, where
# make's echoed command line would land as garbage entries.
cover-pcts:
	@CGO_ENABLED=1 go run ./cmd/covergate -pcts $(COVERPROFILE)

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

# mutate-smoke [PARALLEL=2]: the rules-core gate for CI. The allowlist is
# scoped to the package so the unused-allow check only judges entries the
# run could consume. Exercises the mutator, the suite-kill contract, and
# the run-private build cache lifecycle on every push; the full gate stays
# a release checkpoint behind mutate-full.
mutate-smoke:
	CGO_ENABLED=1 go run ./cmd/mutate -allow .mutate-allow -allow-scope internal/rules -parallel $(or $(PARALLEL),2) -pkgs internal/rules

# mutate-full [PARALLEL=6] [CHALLENGE=1] [LABEL=run] [ARGS="-pkgs ..."]: the
# whole gate driven to a definitive verdict, logging to
# logs/archive/mutate-<label>.log. A host-side crash (non-zero exit with no
# summary line) resumes from the same log automatically, up to 5 attempts; a
# real gate failure (summary present) fails this target immediately with the
# runner's verdict. ARGS scopes the package set for wave-local gates, e.g.
# ARGS="-pkgs internal/clock".
mutate-full:
	@mkdir -p logs/archive; \
	log=logs/archive/mutate-$(or $(LABEL),run).log; : > $$log; \
	for attempt in 1 2 3 4 5; do \
		CGO_ENABLED=1 go run ./cmd/mutate -allow .mutate-allow -parallel $(or $(PARALLEL),6) $(if $(CHALLENGE),-challenge) $(ARGS) -resume $$log >> $$log 2>&1 && exit 0; \
		if grep -q '^mutate: [0-9][0-9]*/[0-9]* run' $$log; then tail -2 $$log; exit 1; fi; \
		echo "mutate-full: attempt $$attempt crashed host-side, resuming from $$log"; \
	done; \
	echo "mutate-full: no definitive verdict after 5 attempts"; tail -3 $$log; exit 1

run:
	CGO_ENABLED=1 go run ./cmd/caro $(or $(ARGS),ports)

migrate:
	CGO_ENABLED=1 go run ./cmd/caro migrate

serve:
	CGO_ENABLED=1 go run ./cmd/caro serve $(ARGS)

# Headless tournament drivers of first-cause.md Scenario 2. Each prints the
# per-series lines and the final leaderboard to stdout and writes the
# per-series txt logs and the summary into their own timestamped folder under
# logs/tourny/ (config.TournamentLogRoot). ARGS
# forwards flags, e.g. make tourney-smoke-10 ARGS="--db scratch.db --parallel 1".
# tourney-resume continues the machine's one interrupted run from its own
# persisted state, e.g. make tourney-resume ARGS="4 --db db/gates-v020.db";
# tourney-close abandons a stalled run instead,
# make tourney-close ARGS="4 --db db/gates-v020.db".
tourney-smoke-32:
	CGO_ENABLED=1 go run ./cmd/caro tourney smoke32 $(ARGS)

tourney-smoke-10:
	CGO_ENABLED=1 go run ./cmd/caro tourney smoke10 $(ARGS)

tourney-smoke-105:
	CGO_ENABLED=1 go run ./cmd/caro tourney smoke105 $(ARGS)

tourney-full:
	CGO_ENABLED=1 go run ./cmd/caro tourney full $(ARGS)

tourney-resume:
	CGO_ENABLED=1 go run ./cmd/caro tourney resume $(ARGS)

tourney-close:
	CGO_ENABLED=1 go run ./cmd/caro tourney close $(ARGS)

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

# darkcontrast checks the shell.css palette pairs against the WCAG formula
# (docs/design-system.md's contrast contract). It lives inside the playground
# module without its own go.mod, so the playground-modules loop never sees it;
# this target is its one entry point, and ci runs it so a token change that
# breaks a pair fails the push.
.PHONY: darkcontrast
darkcontrast:
	CGO_ENABLED=1 go -C playground run ./darkcontrast

ci: playground-modules darkcontrast fmt-check lint vet build test-race cover
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

# db-checkpoint folds every db/*.db write-ahead log into its main file and
# truncates the sidecars. SQLite folds the log only on a clean close, so a
# db left behind by a killed process commits as a stub main plus an ignored
# -wal sidecar holding the real data; this makes the tree committable as one
# honest file per database. Fails if any log is held busy by a live writer.
# The tool lives in its own nested module like logstats (see above).
.PHONY: db-checkpoint
db-checkpoint:
	CGO_ENABLED=1 go -C playground/walckpt build -o ../../bin/walckpt.exe .
	bin/walckpt.exe db

# arena runs the strength-inversion experiment harness: capability-
# perturbed tier engines under the shipped clock law, serial pairings on
# the whole machine. Seat labels and flags in playground/arena/main.go,
# e.g. make arena ARGS="-tc 0 -games 12 -seats hard,medium".
.PHONY: arena
arena:
	CGO_ENABLED=1 go -C playground/arena build -o ../../bin/arena.exe .
	bin/arena.exe $(ARGS)

# clocktune sweeps PID gain grids for one time control over the real clock
# law through the gains seam, ranking cells by worst drain-trajectory
# deviation across cost models and game lengths; the ranked artifact lands
# in playground/clocktune/out/, e.g. make clocktune ARGS="-tc 3".
.PHONY: clocktune
clocktune:
	CGO_ENABLED=1 go -C playground/clocktune build -o ../../bin/clocktune.exe .
	bin/clocktune.exe $(ARGS)

# playground-modules builds every nested playground module: a nested go.mod
# is invisible to the root module's package patterns, so a config rename
# rotting one module escaped every gate until the v0.21 adversarial pair
# caught it. Wired into ci as the blind-spot closer.
.PHONY: playground-modules
playground-modules:
	@for d in playground/*/go.mod; do \
		echo "build $$(dirname $$d)"; \
		CGO_ENABLED=1 go -C $$(dirname $$d) build -o ../../bin/ . || exit 1; \
	done

# gpu-spike builds the nvcc measurement dll and drives it, the toolchain
# and cost measurements behind the CUDA-offload assessment in
# playground/gpuoffload/ANALYSIS.md. Local-only, never in ci: needs an
# MSVC host compiler in PATH (nvcc links through cl.exe) plus the CUDA
# toolkit, e.g. from a shell that ran vcvars64 with the toolkit's bin
# ahead of any older toolkit on PATH.
.PHONY: gpu-spike
gpu-spike:
	@mkdir -p bin
	nvcc -shared -o bin/spike.dll playground/gpuoffload/spike.cu
	CGO_ENABLED=1 go -C playground/gpuoffload build -o ../../bin/gpuoffload.exe .
