# Deploying with Portainer

This guide is for Docker Standalone managed by Portainer. It does not work with Swarm.

| File | Purpose |
|---|---|
| [docker-compose.portainer.yml](docker-compose.portainer.yml) | Stack definition to paste into Portainer |
| [stack.env.example](stack.env.example) | Environment variable template for the stack |

You build and provide the image yourself: `${IMAGE}:${IMAGE_TAG}`. `IMAGE` is the repository without a tag; the project does not preconfigure a registry.

## 1. Build and push the image (dev machine)

```sh
# Replace the registry address and namespace below with your own.
export IMAGE=registry.example.com/your-namespace/db-backup-manager
docker login registry.example.com
make docker-push TAG=v1.0.0        # pushes :v1.0.0 and :latest
```

You can also pass the repository directly: `./scripts/docker-push.sh v1.0.0 "$IMAGE"`. The script does not read `.env`; export `IMAGE` or pass it on the command line. Do not include a tag in `IMAGE`. If no tag is given, the script uses the git short SHA (with `-dirty` appended when there are uncommitted changes). For Docker Hub, use `IMAGE=your-namespace/db-backup-manager` and `docker login`.

To log in automatically, pass both `REGISTRY_USER` and `REGISTRY_PASSWORD` through the environment (e.g. CI secrets). `REGISTRY` is derived from `IMAGE` by default, or `docker.io` for Docker Hub; set it explicitly if needed. Never store credentials in the repository.

The default build platform is `linux/amd64`. Change it with `PLATFORMS=linux/arm64 make docker-push ...` if your server is ARM.

## 2. Add the registry to Portainer (once)

**Registries → Add registry → Custom registry**

- Name: a display name of your choice
- Registry URL: your registry hostname (e.g. `registry.example.com`)
- For a private registry, enable **Authentication** and enter your registry account. Public images need no credentials.

## 3. Create the stack

**Stacks → Add stack**

1. Name: `db-backup-manager`
2. Build method:
   - **Web editor**: paste the contents of [docker-compose.portainer.yml](docker-compose.portainer.yml), or
   - **Repository**: point to the repo, Compose path `deploy/docker-compose.portainer.yml`.
3. **Environment variables → Advanced mode**: paste [stack.env.example](stack.env.example) and fill in at least:
   - `IMAGE`: the repository you just pushed, without a tag (same as step 1)
   - `IMAGE_TAG`: the tag you just pushed, e.g. `v1.0.0`
   - `ADMIN_PASSWORD`: at least 12 characters
   - `POSTGRES_NETWORK`: the network name of your PostgreSQL stack (check with `docker network ls` on the server)
4. **Deploy the stack**.

No need to create directories or `chown` beforehand. See [Data storage](#data-storage).

## 4. Verify and access

- **Containers → db-backup-manager** should become `healthy` within 10–30 seconds. If not, check **Logs**.
- The port is only published on `127.0.0.1`. Access it from your workstation with an SSH tunnel:

  ```sh
  ssh -N -L 8088:127.0.0.1:8088 user@SERVER_IP
  ```

  then open **https://localhost:8088** (typing `http://` also redirects to `https://`). The browser warns about the certificate the first time (self-signed, generated in the data volume): choose **Advanced → Proceed**.
- If you open it directly by server IP/domain name, add them to `TLS_HOSTS` so the certificate matches the address. Details: [README → HTTPS](../README.md#https).
- When adding a database connection in the UI, the host is the **Postgres service/container name** on the shared network (e.g. `postgres`), not `localhost`.

## Data storage

By default (with `DATA_DIR` and `BACKUP_DIR` left empty), the stack uses two **named volumes**:

| Volume | Mount | Contents |
|---|---|---|
| `db-backup-manager-data` | `/data` | `metadata.db`, `connections.key`, `tls/` (HTTPS certificate) |
| `db-backup-manager-backups` | `/backups` | Backup files (`.dump`, …) |

Docker creates the volumes on first deploy and copies ownership from the image (user `postgres`, UID 999), so the container can write to them right away. Removing or redeploying the stack does **not** remove the volumes. They are only lost if you delete them manually under **Volumes**.

Actual location on the server: `/var/lib/docker/volumes/<volume name>/_data`.

### Using host directories (optional)

Only needed if you want backups on a specific disk/directory. Set `DATA_DIR` and `BACKUP_DIR` to **absolute paths** and create them first:

```sh
sudo install -d -o 999 -g 999 -m 700 /opt/db-backup-manager/data /opt/backups/db
```

They must be created first because, with bind mounts, Docker creates missing directories owned by `root`, while the container runs as UID 999 and cannot write to them (`permission denied` when opening `metadata.db`).

If it still fails after assigning UID 999, check:

- **Relative paths** (`./data`): in Portainer, this path does not point to the directory you `chown`ed. Always use absolute paths.
- **Existing files inside** owned by another user (copied from another machine, or previously run as root): `chown` needs `-R`.

  ```sh
  sudo chown -R 999:999 /opt/db-backup-manager/data /opt/backups/db
  ```

- **A parent directory** that UID 999 cannot traverse (e.g. a root-owned parent with mode `700`).
- **Rootless Docker or `userns-remap`**: container UIDs map to different UIDs on the host. Use named volumes in that case.
- **SELinux** (RHEL/CentOS/Rocky): add the `:Z` suffix to the volume, e.g. `/opt/backups/db:/backups:Z`.

## Upgrading

1. Dev machine: `IMAGE=registry.example.com/your-namespace/db-backup-manager make docker-push TAG=v1.0.1` (replace with your repository)
2. Portainer: **Stacks → db-backup-manager → Editor**, change `IMAGE_TAG=v1.0.1` → **Update the stack**.
   - If you use `latest`, enable **Re-pull image and redeploy**.

Data in the volumes is preserved.

## Backing up the tool itself

Back up **both** `metadata.db` and `connections.key` (in the data volume). Without `connections.key`, stored connection passwords and cloud configs cannot be decrypted.

```sh
docker run --rm -v db-backup-manager-data:/data:ro -v "$PWD":/out debian:bookworm-slim \
  tar czf /out/dbbm-data-$(date +%F).tgz -C /data .
```

## Troubleshooting

- **Forgot the admin password**: **Containers → db-backup-manager → Console**, run `db-backup-manager reset-admin-password`.
- **`network postgres_network not found`**: wrong `POSTGRES_NETWORK`. Double-check with `docker network ls`.
- **`pull access denied` / `unauthorized`**: the registry was not added (step 2) or the credentials are wrong.
- **Behind an HTTPS reverse proxy**: set `TLS_MODE=off`, `COOKIE_SECURE=true` and let the proxy connect to the container's port.
