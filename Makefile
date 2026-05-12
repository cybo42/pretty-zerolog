.PHONY: build snapshot release clean test tidy

BINARY := pretty-zerolog

build:
	go build -o $(BINARY) ./

test:
	go test ./...

tidy:
	go mod tidy

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

clean:
	rm -rf dist/ $(BINARY)
