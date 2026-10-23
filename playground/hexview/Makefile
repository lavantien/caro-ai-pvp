CGO_ENABLED ?= 1

.PHONY: fmt vet test build render clean

fmt:
	gofmt -l -w .

vet:
	CGO_ENABLED=$(CGO_ENABLED) go vet ./...

test: fmt vet
	CGO_ENABLED=$(CGO_ENABLED) go test ./...

build: fmt vet
	CGO_ENABLED=$(CGO_ENABLED) go build -trimpath -ldflags "-s -w" -o hexview.exe .

render: build
	./hexview.exe -out out

clean:
	rm -f hexview.exe
