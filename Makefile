.PHONY: all build test clean run export-pages release

unexport GOROOT
BINARY_NAME=bin/ag-stats

all: test build

build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) main.go
	@echo "Build complete: $(BINARY_NAME)"

test:
	go test -v ./...

run: build
	./$(BINARY_NAME) serve --port 58763 --open

run-auth: build
	./$(BINARY_NAME) serve --port 58763 --token "secret123" --open

stats: build
	./$(BINARY_NAME) stats --limit 10

export-pages:
	@mkdir -p docs
	cp -r internal/web/static/* docs/
	@echo "Static dashboard exported to docs/ for GitHub Pages."

release:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -o dist/ag-stats-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build -o dist/ag-stats-linux-arm64 main.go
	GOOS=darwin GOARCH=amd64 go build -o dist/ag-stats-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build -o dist/ag-stats-darwin-arm64 main.go
	GOOS=windows GOARCH=amd64 go build -o dist/ag-stats-windows-amd64.exe main.go
	@echo "Cross-platform binaries built successfully in dist/"

clean:
	rm -rf bin/ dist/
