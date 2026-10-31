.PHONY: all build test clean run release

unexport GOROOT
BINARY_NAME=bin/ag-stats
BUILD_FLAGS=-trimpath -ldflags="-s -w"

all: test build

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(BINARY_NAME) main.go
	@echo "Build complete: $(BINARY_NAME)"

test:
	go test -v ./...

run: build
	./$(BINARY_NAME) serve --port 58763 --open

run-auth: build
	./$(BINARY_NAME) serve --port 58763 --token "secret123" --open

stats: build
	./$(BINARY_NAME) stats --limit 10

release:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/ag-stats-linux-amd64 main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/ag-stats-linux-arm64 main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/ag-stats-darwin-amd64 main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) -o dist/ag-stats-darwin-arm64 main.go
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) -o dist/ag-stats-windows-amd64.exe main.go
	@echo "Cross-platform lightweight binaries built successfully in dist/"

clean:
	rm -rf bin/ dist/
