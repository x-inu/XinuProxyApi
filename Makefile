.PHONY: management backend build test verify clean

VERSION ?= dev

management:
	VERSION=$(VERSION) ./scripts/build-management.sh

backend:
	go build -o cli-proxy-api ./cmd/server

build: management backend

test:
	cd web/management-center && bun run test
	go test ./...

verify:
	./scripts/verify.sh

clean:
	rm -rf static/management.html web/management-center/dist cli-proxy-api test-output
