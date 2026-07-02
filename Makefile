GOPROXY ?= https://goproxy.cn,direct
CGO_ENABLED ?= 0
BINARY := nexus-cli

.PHONY: build test vet clean run-help

build:
	CGO_ENABLED=$(CGO_ENABLED) GOPROXY=$(GOPROXY) go build -o $(BINARY) ./cmd/nexus-cli

test:
	CGO_ENABLED=$(CGO_ENABLED) GOPROXY=$(GOPROXY) go test ./...

vet:
	CGO_ENABLED=$(CGO_ENABLED) GOPROXY=$(GOPROXY) go vet ./...

run-help: build
	./$(BINARY) --help

clean:
	rm -f $(BINARY)
