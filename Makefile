.PHONY: test build web run docker-push
export GOMODCACHE := $(CURDIR)/.cache/go-mod
export GOCACHE := $(CURDIR)/.cache/go-build
test:
	go test -race ./...
	go vet ./...
web:
	cd web && npm ci && npm run build
build: web
	mkdir -p bin
	go build -trimpath -o bin/db-backup-manager ./cmd/server
run:
	go run ./cmd/server
docker-push:
	./scripts/docker-push.sh $(TAG)
