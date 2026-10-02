CGO_ENABLED ?= 1

.PHONY: fmt vet calibrate bench

fmt:
	gofmt -l -w .

vet:
	CGO_ENABLED=$(CGO_ENABLED) go vet ./...

# Short run for wiring checks: 100k rows per phase, under a minute total.
calibrate: fmt vet
	CGO_ENABLED=$(CGO_ENABLED) go run . -all -rows 100000

# Full run: 1M rows per insertion phase, per candidate.
bench: fmt vet
	CGO_ENABLED=$(CGO_ENABLED) go run . -all
