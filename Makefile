.PHONY: management backend build test verify clean

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.Version=$(VERSION) \
	-X main.Commit=$(COMMIT) \
	-X main.BuildDate=$(BUILD_DATE)

management:
	VERSION=$(VERSION) ./scripts/build-management.sh

backend:
	go build -ldflags "$(LDFLAGS)" -o xinu-proxy-api ./cmd/server

build: management backend

test:
	cd web/management-center && bun run test
	go test ./...

verify:
	./scripts/verify.sh

clean:
	rm -rf static/management.html web/management-center/dist xinu-proxy-api test-output
