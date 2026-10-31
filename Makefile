VERSION := $(shell git describe --tags --always --dirty)

.PHONY: test build run

test:
	go test ./...

# shortcut: no frontend yet, S12 adds the frontend build before go build
build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/nudged .

run: build
	./bin/nudged
