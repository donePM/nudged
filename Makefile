VERSION := $(shell git describe --tags --always --dirty)

.PHONY: test build run

test:
	go test ./...

build:
	[ -d frontend/node_modules ] || npm --prefix frontend ci
	npm --prefix frontend run build
	go build -ldflags "-X main.version=$(VERSION)" -o bin/nudged .

run: build
	./bin/nudged
