#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
name="dbbackup-test-$$"
cleanup() { docker rm -fv "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker run -d --name "$name" -e POSTGRES_PASSWORD=integration-only-password \
  -p 127.0.0.1::5432 postgres:18-bookworm >/dev/null
tries=0
until docker exec "$name" pg_isready -U postgres >/dev/null 2>&1; do
  tries=$((tries + 1))
  if [ "$tries" -ge 60 ]; then echo "PostgreSQL startup timeout" >&2; exit 1; fi
  sleep 1
done
mapping=$(docker port "$name" 5432/tcp)
TEST_PGPORT="${mapping##*:}" go test -race -v ./...
