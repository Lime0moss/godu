BINARY := godu
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build clean run install test lint

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/godu

run: build
	./$(BINARY) .

install:
	go install $(LDFLAGS) ./cmd/godu

clean:
	rm -f $(BINARY) $(BINARY)-*
	go clean

test:
	go test ./...

lint:
	golangci-lint run ./...

# Build for multiple platforms
.PHONY: release
release:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/godu
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/godu
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/godu
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/godu
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe ./cmd/godu
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-windows-arm64.exe ./cmd/godu
