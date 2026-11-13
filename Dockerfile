# PostgreSQL clients must be at least the server's major version.
ARG PG_MAJOR=18
FROM node:22-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
# Server message translations are shared with the Go code.
COPY internal/manager/server_messages.json /src/internal/manager/
RUN npm run build

FROM golang:1.27-trixie AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

# rclone uploads encrypted archives to Google Drive, OneDrive, S3… Built from
# source at a pinned release so it uses our (patched) Go toolchain, with
# vulnerable dependencies bumped to fixed versions. The source and all modules
# are verified against the Go checksum database (sum.golang.org).
FROM golang:1.27-trixie AS rclone
ARG RCLONE_VERSION=1.75.2
WORKDIR /src
RUN set -eu; \
    dir=$(go mod download -json github.com/rclone/rclone@v${RCLONE_VERSION} | sed -n 's/^\t"Dir": "\(.*\)",$/\1/p'); \
    cp -r "$dir"/. . && chmod -R u+w . \
    && go get github.com/go-chi/chi/v5@v5.5.0 golang.org/x/net@v0.62.0 \
       golang.org/x/crypto@v0.58.1-0.20261009181718-2f1f834330b2 \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/rclone/rclone/fs.Version=v${RCLONE_VERSION}" -o /out/rclone .

# Only database client tools are needed (pg_dump/pg_restore, mariadb-dump/
# mariadb, mongodump/mongorestore), so use a minimal Alpine base instead of a
# database server image: far fewer packages, and Alpine ships security fixes
# quickly. SQL Server needs no client tool: it is driven over TDS with
# BACKUP/RESTORE DATABASE through a shared directory. mariadb-connector-c
# ships the auth plugins (caching_sha2_password…) MySQL 8+ servers require. The postgres user is moved
# to UID/GID 999 (as in the official postgres image) so existing volumes stay writable.
FROM alpine:3.24
ARG PG_MAJOR
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates tzdata postgresql${PG_MAJOR}-client mariadb-client mariadb-connector-c mongodb-tools \
    && sed -i 's/^postgres:x:70:70:/postgres:x:999:999:/' /etc/passwd \
    && sed -i '/^ping:x:999:/d; s/^postgres:x:70:/postgres:x:999:/' /etc/group
RUN mkdir -p /data /backups /app/web && chown -R postgres:postgres /data /backups /app
COPY --from=rclone /out/rclone /usr/local/bin/rclone
COPY --from=backend /server /usr/local/bin/db-backup-manager
COPY --from=web /src/web/dist /app/web
# HTTPS with a self-signed certificate kept in /data/tls (TLS_MODE=off for plain HTTP).
ENV LISTEN_ADDR=0.0.0.0:8080 TLS_MODE=self-signed WEB_DIR=/app/web BACKUP_DIR=/backups METADATA_DB=/data/metadata.db TZ=Asia/Ho_Chi_Minh
USER postgres
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/db-backup-manager"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s CMD ["/usr/local/bin/db-backup-manager", "healthcheck"]
