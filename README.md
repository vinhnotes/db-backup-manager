# DB Backup Manager

A self-hosted web app for managing backups and restores of PostgreSQL, MySQL/MariaDB, MongoDB, SQL Server and file directories. Manage multiple connections, schedule backups, keep encrypted copies in the cloud and track results in one place.

## Contents

- [Features](#features)
- [Quick start with Docker Compose](#quick-start-with-docker-compose)
- [Supported databases](#supported-databases)
- [Configuration and deployment](#configuration-and-deployment)
- [Users and permissions](#users-and-permissions)
- [Cloud storage and encryption](#cloud-storage-and-encryption)
- [Notifications, restore testing and monitoring](#notifications-restore-testing-and-monitoring)
- [Operating procedure](#operating-procedure)
- [Development and testing](#development-and-testing)
- [Limitations](#limitations)
- [Contributing](#contributing)
- [Security and private data](#security-and-private-data)
- [License](#license)

## Features

- Manage multiple database connections and file directories from a web UI; test each connection before saving.
- Manual or cron-scheduled backups, retention management, archive upload/download and SHA-256 verification.
- Restore into a new database or directory; configure a separate restore account per connection.
- Restore testing, manual or after each scheduled backup, with results shown next to each backup.
- Offsite storage via rclone: Google Drive, OneDrive, Dropbox, S3/R2/MinIO, B2, SFTP, WebDAV and other supported backends. Encrypted with age before upload, with separate cloud retention.
- Notifications via Telegram, Slack, Discord, SMTP or webhook; a `/metrics` endpoint for Prometheus.
- Automatic retries for failed backups/cloud uploads and catch-up of missed schedules when the app restarts.
- User management, per-connection/per-database permissions and an activity log; customizable name, logo and schedule timezone.
- HTTPS with an auto-generated or your own certificate. The container runs as a non-root user, supports a read-only root filesystem and needs neither the Docker socket nor the Portainer API.
- UI in English, Vietnamese and Japanese.

## Quick start with Docker Compose

### Requirements

- Docker Engine or Docker Desktop with Docker Compose v2.
- A database reachable from the container, or a mounted directory for file backups.
- Write access to the metadata and backup directories. The container runs as UID/GID `999:999`.

After cloning the repository, run the following from the repository root. Compose builds the image from source; no registry account is needed.

```sh
cp .env.example .env
chmod 600 .env
docker network ls
```

Edit `.env` and set at least:

```dotenv
ADMIN_USER=admin
ADMIN_PASSWORD=REPLACE_WITH_A_STRONG_UNIQUE_PASSWORD
POSTGRES_NETWORK=your-existing-database-network
```

`ADMIN_PASSWORD` must be 12–72 bytes long. `POSTGRES_NETWORK` is the name of the actual Docker network your database container uses. If there is no shared network yet, create one with `docker network create db-backup`, set `POSTGRES_NETWORK=db-backup` and attach your database container to it. Despite the name `POSTGRES_NETWORK`, the network can be used for any database type.

With the default paths `./data` and `./backups`, create the directories with the right ownership on a Linux host:

```sh
sudo install -d -o 999 -g 999 -m 700 ./data ./backups
docker compose up -d --build
docker compose ps
```

Open **https://localhost:8088**, accept or trust the self-signed certificate generated on first run, and sign in with the account set in `.env`. If the app runs on a remote server, see [access via SSH tunnel](#3-access-via-ssh-tunnel).

Under **Databases → Add connection**, choose the database type, enter the connection details, then click **Test connection** and **Save**. For a database on the same Docker network, use the service/container name as the host. For file directories, see [Files](#files-upload-directories).

Run a manual backup and test a restore before setting up schedules. To diagnose startup errors:

```sh
docker compose logs --tail=100 db-backup-manager
```

Optional variables are listed in [.env.example](.env.example). To deploy with Portainer or push the image to your own registry, see the [deployment guide](deploy/README.md).

## Supported databases

Choose the **Database type** when adding a connection. Each backup remembers its database type and can only be restored to a server of the same type.

| Type | Backup method | Downloaded file | Restore |
|---|---|---|---|
| PostgreSQL | `pg_dump --format=custom`, verified with `pg_restore --list` | `.dump` | `CREATE DATABASE` + `pg_restore --single-transaction --no-owner` |
| MySQL / MariaDB | `mariadb-dump --single-transaction --routines --triggers --events`, gzip-compressed, checked for the `-- Dump completed` line | `.sql.gz` | `CREATE DATABASE` + `mariadb` client; `DEFINER` is stripped so objects belong to the restore account |
| MongoDB | `mongodump --db --archive --gzip`, archive header checked | `.archive.gz` | Refused if the target database exists, then `mongorestore --nsFrom/--nsTo` |
| SQL Server | `BACKUP DATABASE … WITH COPY_ONLY, CHECKSUM` (+ `COMPRESSION` on non-Express editions) and `RESTORE VERIFYONLY` | `.bak` | `RESTORE DATABASE … WITH MOVE` to new files in the default data directory |
| Files (directory) | The mounted directory is packed into a tar.gz, then the whole archive is read back to verify it | `.tar.gz` | Extracted into a **new** directory inside the restore directory; never overwrites |

Uploads accept the format of the selected server (MySQL also accepts uncompressed `.sql`). Passwords never appear on the command line: client tools read them from a temporary `0600` file in `/tmp`, deleted right after use.

### MySQL / MariaDB

The image ships the MariaDB 11.8 client, which works with MySQL 5.7/8.x and MariaDB 10.x/11.x. SSL mode `prefer` uses TLS when the server supports it (without certificate verification); `verify-full` verifies both certificate and hostname.

```sql
CREATE USER 'backup_user'@'%' IDENTIFIED BY 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
GRANT SELECT, SHOW VIEW, TRIGGER, EVENT, LOCK TABLES ON *.* TO 'backup_user'@'%';
-- Optional, for restore: create new databases with full rights on them
CREATE USER 'restore_user'@'%' IDENTIFIED BY 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
GRANT ALL PRIVILEGES ON *.* TO 'restore_user'@'%';
```

Backing up routines on MySQL 8 also requires `SHOW_ROUTINE` (or global `SELECT`) if the user is not the definer. Non-InnoDB tables (MyISAM) are not consistent while the database is being written to, because `--single-transaction` only applies to InnoDB.

### MongoDB

The **Authentication database** field is `authSource` (default `admin`). Leave the username empty if MongoDB has authentication disabled. Connections target a single host/port; `mongodb+srv://` URIs (Atlas) and custom replica set options are not supported yet.

```js
db.getSiblingDB("admin").createUser({ user: "backup_user", pwd: "…", roles: ["backup"] })
// Optional, for restore:
db.getSiblingDB("admin").createUser({ user: "restore_user", pwd: "…", roles: ["restore", "readAnyDatabase"] })
```

Backing up a single database does not include users/roles defined in that database.

### SQL Server

SQL Server has no remote dump tool: `BACKUP DATABASE` writes the `.bak` file **on the SQL Server machine**. Each SQL Server connection therefore needs a **shared directory** between SQL Server and this container, declared with two paths:

- **Path on SQL Server**: the directory as SQL Server sees it, e.g. `/var/opt/mssql/backup` or `D:\Backup`.
- **Path in this container**: the same directory mounted into the manager container, e.g. `/mssql-backup`.

The manager writes the `.bak` there, moves it into `BACKUP_DIR`, then deletes it from the shared directory. On restore, the manager copies the backup into the shared directory (mode `0644`) for SQL Server to read, and deletes it afterwards. Example on a single Docker host:

```yaml
services:
  mssql:
    volumes:
      - mssql-backup:/var/opt/mssql/backup
  db-backup-manager:
    group_add:
      - "10001" # mssql group: SQL Server creates .bak files with mode 0640
    volumes:
      - mssql-backup:/mssql-backup
volumes:
  mssql-backup:
```

Permission requirements (tested with SQL Server 2022 on Linux):

- SQL Server (user `mssql`, UID/GID 10001) creates `.bak` files with mode `0640`, so the manager container (UID 999) must belong to group 10001 (`group_add`) to read them.
- The directory must be writable by both sides, e.g. `chmod 0777`, or `2775` with group 10001. **Do not use the sticky bit** (`1777`): the manager would be unable to delete files created by SQL Server.
- On separate machines, use an SMB/NFS share mounted on both sides with equivalent permissions. **Test connection** checks the login and write access to the directory inside the container, but only the first backup confirms that both paths point to the same place.

```sql
CREATE LOGIN backup_user WITH PASSWORD = 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
-- In each database to back up:
CREATE USER backup_user FOR LOGIN backup_user;
ALTER ROLE db_backupoperator ADD MEMBER backup_user;
-- To list database sizes (optional):
GRANT VIEW ANY DEFINITION TO backup_user;
-- Optional, for restore:
CREATE LOGIN restore_user WITH PASSWORD = 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
ALTER SERVER ROLE dbcreator ADD MEMBER restore_user;
```

Backups use `COPY_ONLY`, so they do not disturb existing differential/log backup chains. `RESTORE VERIFYONLY` only runs when the backup account has CREATE DATABASE permission; otherwise verification relies on `BACKUP … WITH CHECKSUM`. A restored database keeps the users/owner stored in the backup; grant application permissions after checking it.

### Files (upload directories)

Back up a file directory, such as a web app's upload folder, with the same schedules, retention, cloud storage, notifications and restore testing as databases. Each **Files** connection is **one directory**. To back up several directories, create several connections.

The container can only read directories mounted into it. Add volumes to `docker-compose.yml`:

```yaml
    volumes:
      - /var/www/app/uploads:/sources/uploads:ro   # source directory, read-only
      - ./restore:/restore                         # restore directory (optional)
```

Then add a **Files** connection: **Source folder** `/sources/uploads`, **Restore folder** `/restore` (leave empty if you don't need restores).

- The container runs as UID 999: source files must be readable by this UID (default `644`/`755` permissions usually are). The restore directory must be writable by UID 999, e.g. `chown 999:999 ./restore`.
- Backups keep the directory structure, permissions (without setuid/setgid), modification times and symlinks. Ownership (UID/GID) is not kept. Sockets, devices and pipes are skipped.
- Restore always creates a new subdirectory `/restore/<target name>` and refuses if it already exists. After checking it, copy the files back yourself, e.g. `rsync -a /path/restore/<name>/ /var/www/app/uploads/`.
- Uploaded or cloud-downloaded archives are validated first: absolute paths, `..`, hardlinks and devices are rejected. Symlinks are created last so no file is ever written through a symlink.
- Each backup packs the **entire** directory (not incremental). Suitable for directories up to a few tens of GB. Very large directories may require a higher `JOB_TIMEOUT`.
- Files being written during a backup may be saved only partially. A file that shrinks while being read fails the backup, which is then retried. Schedule file backups right after database backups so both copies are close in time.
- The source directory must not contain (or be inside) `BACKUP_DIR`, and must not be the same as the restore directory.

## Configuration and deployment

### 1. Prepare PostgreSQL

Find the Docker network name and the DNS alias/service of your existing PostgreSQL. Do not expose the PostgreSQL port to the Internet. The manager must be able to reach the host/port of each connection; for PostgreSQL in Docker, use the same external network and the container's DNS alias.

Create a backup user with a PostgreSQL admin account. Example (replace the password before running):

```sql
CREATE ROLE backup_user LOGIN PASSWORD 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
GRANT pg_read_all_data TO backup_user;
GRANT CONNECT ON DATABASE app_db TO backup_user;
```

`pg_read_all_data` does not bypass Row Level Security. If the database uses RLS, large objects, extensions or special ACLs, the DBA must grant appropriate permissions and test backups for each database. Do not grant superuser by default. To compute the size of all databases, the user needs CONNECT on each managed database (or suitable monitoring privileges).

An optional restore account, separate from the backup account:

```sql
CREATE ROLE restore_user LOGIN CREATEDB PASSWORD 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
GRANT CONNECT ON DATABASE postgres TO restore_user;
```

Restored objects are owned by the restore account (`--no-owner --no-privileges`). Extensions that need superuser or special configuration may make the restore fail. Grant application permissions after the DBA has verified the target data.

### 2. Configure and build

On the machine running the app, in the project directory:

```bash
cp .env.example .env
chmod 600 .env
```

Set `ADMIN_PASSWORD` and `POSTGRES_NETWORK` in `.env`, then sign in and go to **Databases → Add connection**. Enter a display name, host, port, username, password, maintenance database (default `postgres`) and SSL mode. Click **Test connection**, then **Save**. You can save multiple servers and pick a connection to view its databases; each backup or schedule is tied to the selected connection. "All databases" applies to that server.

The host is resolved from the server/container running the manager. `localhost` inside the container does not point to your workstation. Connections are stored in SQLite and passwords are encrypted with the `connections.key` file next to the metadata file. Back up **both the metadata and the key file** together to be able to restore your configuration. Do not delete the key file while saved connections exist. Viewers have read-only access; admins manage connections. A connection with running/queued jobs cannot be edited or deleted; remove its schedules before deleting the connection. Archives and history are kept when a connection is deleted.

Connections created in the UI are used for listing databases, backups, schedules and restores. Under **Databases → Add/Edit connection**, enter a separate **Restore account** and **Restore password** for that server; leave the account empty to disable restores. The account needs LOGIN, CREATEDB and CONNECT on the maintenance database. **Test connection** verifies both the backup connection and the CREATEDB permission of the restore account. The check does not guarantee that every extension in an archive can be restored.

In **Backups**, filter by server then database; uploads attach the archive to the selected server. Click restore on an archive, choose a target server with restore configured (it may differ from the source server), enter a new database name, type it again to confirm and confirm you trust the archive. Existing databases are never overwritten. Archives of deleted servers remain available to download or restore to another server. Backup and restore passwords are encrypted; leave a password empty when editing to keep the stored value, or choose to clear it.

All database connection settings (host, account, password, restore, SSL) are managed only in the web UI; `.env` no longer holds database credentials. Variables such as `PGHOST`, `PGUSER`, `PGPASSWORD`, `RESTORE_PGUSER`… are ignored if still set.

`.env` variables:

| Variable | Meaning |
| --- | --- |
| `ADMIN_USER`, `ADMIN_PASSWORD` | Admin account. `ADMIN_PASSWORD` is required (12–72 bytes, use a strong password) and remains the admin password **until** it is changed in the web UI; see [Admin password](#admin-password) |
| `VIEWER_PASSWORD` | Optional; a read-only `viewer` account (all connections, no downloads); must differ from the admin password |
| `POSTGRES_NETWORK` | Required name of an existing external Docker network (`docker network ls`); no default |
| `PG_MAJOR` | Client version in the image, default `18`; must be >= the server major version |
| `DATA_DIR`, `BACKUP_DIR` | Host directories for metadata and archives; absolute paths recommended on a VM |
| `TZ` | Default schedule timezone (`Asia/Ho_Chi_Minh`). Admins can change it in the UI (account menu at top right → Change schedule timezone); the UI value takes precedence |
| `JOB_TIMEOUT` | Per-job timeout, default `2h` |
| `MAX_UPLOAD_BYTES` | Default `10737418240` (10 GiB) |
| `RCLONE_BINARY` | Optional rclone path; defaults to `rclone` in `PATH` (included in the image) |

The image runs as user `postgres` (UID/GID 999:999). Create new directories with matching ownership, e.g. for the paths below (also update both variables in `.env`):

```bash
sudo install -d -o 999 -g 999 -m 700 /opt/db-backup-manager/data /opt/backups/db
```

Do not grant access to the archive directory to users who don't need it. If you use the default `./data` and `./backups`, also create them with the right ownership before starting. Do not use `chmod 777`.

```bash
docker compose build
docker compose up -d
docker compose ps
docker compose logs --tail=100 db-backup-manager
```

Compose only publishes `127.0.0.1:8088:8080`, uses a read-only root filesystem, drops Linux capabilities and does not mount the Docker socket. The healthcheck only checks the backend/SQLite; database status is shown separately on the dashboard.

### 3. Access via SSH tunnel

On your workstation:

```bash
ssh -N -L 8088:127.0.0.1:8088 ubuntu@SERVER_IP
```

Open **https://localhost:8088** and sign in with the account set in `.env`.

### HTTPS

The Docker image runs HTTPS by default (`TLS_MODE=self-signed`). On first start, the server generates a self-signed certificate (ECDSA P-256, valid for 10 years) and stores it in `DATA_DIR/tls/` (`cert.pem`, `key.pem`), so redeploying keeps the same certificate. Requests over `http://` are redirected to `https://` (same port). Browsers warn the first time because the certificate is not CA-signed: choose **Advanced → Proceed** (or import `cert.pem` on your machine to remove the warning).

The certificate is always valid for `localhost`, `127.0.0.1` and the container hostname. If you access the server by IP or domain name, add it to `TLS_HOSTS` (comma-separated), e.g. `TLS_HOSTS=192.0.2.10,backup.example.com`. When `TLS_HOSTS` changes, or the certificate has less than 30 days left, the server regenerates it on startup.

| `TLS_MODE` | Meaning |
|---|---|
| `self-signed` | Default in Docker. Generates and keeps a certificate. |
| `file` | Use your own certificate: mount the files into the container and set `TLS_CERT_FILE`, `TLS_KEY_FILE`. |
| `off` | Plain HTTP, e.g. behind an HTTPS reverse proxy (then also set `COOKIE_SECURE=true`). |

The session cookie automatically gets `Secure` when TLS is on; `COOKIE_SECURE` is only needed to override it. Running locally with `make run` stays on HTTP unless `TLS_MODE` is set.

### 4. Portainer

See the full guide in [deploy/README.md](deploy/README.md): pass your repository via `IMAGE` when pushing/deploying the image, create the stack from [deploy/docker-compose.portainer.yml](deploy/docker-compose.portainer.yml); it uses named volumes, so no manual directory creation or `chown` is needed.

## Users and permissions

The admin from `.env` (`ADMIN_USER`) always has full access. Other accounts are created in the UI: user menu at top right → **Manage users** (`#/users`).

- **Administrator**: full access like the `.env` admin — connections, cloud destinations, users, all databases.
- **User**: only sees connections, databases, backups, schedules and jobs within the granted scope. Each permission row is *Connection* (or all) × *Database* (empty = all) × *Level*:
  - **View**: view backups, schedules, jobs.
  - **Operate**: run backups, download, upload, manage schedules, delete backups, push to cloud.
  - Backups/schedules for "All databases" and restores (into the target connection) require Operate on **all** databases of that connection.
- Managing connections and cloud destinations, and pulling files from the cloud, is limited to administrators; browsing cloud files requires View on the entire connection.
- Users change their own password in **Profile & password**. The `.env` admin can also change it there; the new password is stored in the metadata and persists even if `ADMIN_PASSWORD` changes or the app restarts.

### Admin password

`ADMIN_PASSWORD` in `.env` is the admin's default password, not just a first-run value:

| Situation | Admin password used to sign in |
| --- | --- |
| Never changed in the web UI | `ADMIN_PASSWORD` from `.env`. Edit `.env` and run `docker compose up -d` to change it |
| Changed in **Profile & password** | The new password (stored in metadata). `ADMIN_PASSWORD` is ignored, even after editing `.env` or restarting |
| After running `reset-admin-password` | Back to `ADMIN_PASSWORD` from `.env` |

Even when ignored, `ADMIN_PASSWORD` is still **required** (12–72 bytes), otherwise the server won't start. A password changed in the web UI lives in `data/metadata.db`: restoring metadata from an old backup also restores the admin password from that point in time.

### Forgot the admin password

The reset command removes the password set in the web UI so that `ADMIN_PASSWORD` from `.env` takes effect again. No restart needed; open admin sessions are signed out.

```bash
docker compose exec db-backup-manager db-backup-manager reset-admin-password
```

If you don't remember `ADMIN_PASSWORD` either: change it in `.env`, run `docker compose up -d` so the container picks up the new value, then run the reset command above.

Outside Docker: `METADATA_DB=data/metadata.db ./bin/db-backup-manager reset-admin-password`. Portainer: open the container's Console and run `db-backup-manager reset-admin-password`.

## Cloud storage and encryption

The Docker image includes [rclone](https://rclone.org), built from a pinned source version with dependencies verified via the Go checksum database. When running locally, install rclone in `PATH` or set `RCLONE_BINARY`.

### Flow

1. A backup completes (manual, or scheduled with a cloud destination selected) → an **Upload to cloud** job is created.
2. The job verifies the archive's SHA-256, encrypts it with [age](https://age-encryption.org) into a temporary file in `BACKUP_DIR`, uploads it with `rclone copyto`, checks the size on the cloud, then deletes the temporary file.
3. Cloud path: `<folder>/<connection name>/<database>/<database>_<YYYYMMDDTHHMMSSZ>_<id>.dump.age`.
4. Cloud retention (per destination, `0` = keep forever) runs with the cleanup job and **always keeps the latest copy** of each database. Local retention does not affect cloud copies.

### Add a cloud destination

Go to **Cloud storage → Add destination**:

- **Google Drive — web OAuth**: choose **Google Drive → Sign in with Google OAuth → Connect Google Drive**, sign in/grant access, then click **Save**. No rclone needed on your workstation. An administrator configures the OAuth Client once, as described below. Edit the destination and click **Reconnect Google Drive** when the token is no longer valid.
- **Manual rclone config — Google Drive / OneDrive / Dropbox** (OAuth): on a machine with a browser, run `rclone config` → `n` → choose `drive` / `onedrive` / `dropbox` and sign in. For Google Drive, the `drive.file` scope is recommended. Then run `rclone config show <remote-name>` and paste the output into the **rclone config** field. Tokens are refreshed automatically; the new token is encrypted and saved after each run.
- **S3 / R2 / MinIO / Wasabi**: paste directly, e.g.:

  ```ini
  type = s3
  provider = AWS
  access_key_id = AKIA...
  secret_access_key = ...
  region = ap-southeast-1
  ```

  The target folder starts with the bucket name, e.g. `my-bucket/db-backups`. Use an access key scoped to the backup bucket only.

The rclone config (including tokens) is encrypted with AES-GCM using `connections.key` and is never returned to the browser. `local`/`alias`/`crypt`… remotes and the `ssh` option are rejected. Click **Test** to write, read and delete a test file on the cloud.

### Google Drive web OAuth over an SSH tunnel

The app uses [Google's OAuth 2.0 for Web Server Applications](https://developers.google.com/identity/protocols/oauth2/web-server). rclone still runs on the server to transfer files and refresh access tokens. The server does not need to be reachable from the Internet; both the server and the browser need outbound access to Google.

1. In Google Cloud Console, enable the **Google Drive API** for the project. Configure the Google Auth Platform (Audience / Data Access), then create an OAuth Client of type **Web application**.
2. Register an **Authorized redirect URI** that exactly matches `GOOGLE_OAUTH_REDIRECT_URL`, including scheme, hostname, port and the path `/api/destinations/google/callback`. Do not use a LAN IP as the callback; use `localhost` over an SSH tunnel. `localhost` and `127.0.0.1` are different hostnames and must match the registered URL.
3. Add three variables to `.env` (Docker Compose already passes them into the container), e.g. with the default Docker HTTPS on host port 8088:

   ```dotenv
   GOOGLE_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
   GOOGLE_OAUTH_CLIENT_SECRET=your-client-secret
   GOOGLE_OAUTH_REDIRECT_URL=https://localhost:8088/api/destinations/google/callback
   ```

   Keep `.env` private and never commit the Client Secret. Restart/recreate the container after changing the configuration. When running the binary directly, pass these variables through the process environment.

4. Open a tunnel from your workstation:

   ```bash
   ssh -N -L 8088:127.0.0.1:8088 user@server
   ```

   Open **https://localhost:8088** in the browser, accept/trust the internal certificate if self-signed, sign in to the app and connect Google Drive. The server-side port in the example is the published Docker port (`127.0.0.1:8088` → container `8080`). If the local port is taken, change the local port, the callback URL and the Google registration to match.

   If the app runs over HTTP: set `TLS_MODE=off`, `COOKIE_SECURE=false` and the callback `http://localhost:8088/api/destinations/google/callback`. If the binary listens on another port, change the tunnel target accordingly. No extra tunnel is needed for rclone's port 53682.

5. Choose the access scope in the form:
   - **Only files created by this app** (`drive.file`, default): limits access to files this OAuth Client may access. Suitable for new backups.
   - **Entire Drive** (`drive`): use when you need folders or older backups created by another OAuth Client (e.g. rclone's default client). This scope allows reading/editing every Drive file; Google may require app verification depending on your deployment. Switching clients and using `drive.file` does not grant access to files created by the old client.
6. Once Google reports success, close the Google window, return to the form, click **Test** if needed, then **Save**. The new grant does not replace the saved config until you click Save. When reconnecting an existing Drive destination, the app keeps the configured `root_folder_id` and `team_drive`. Select the right existing folder (or a Folder ID via manual rclone config) when you need to browse existing backups.

The authorization session waits up to 10 minutes; the pending result is kept for up to 30 minutes, bound to the admin's login session, and lost on server restart. Testing a destination does not consume the OAuth result; a successful save does. Tokens and the Client Secret are never returned to the browser; the Drive config is encrypted with `connections.key` like a manual rclone config. Keeping the Client ID/Secret alongside the token lets rclone keep refreshing after a server restart.

If the Audience is **External** and the Publishing status is still **Testing**, Drive refresh tokens expire after **7 days**, even with frequent backups. Switch to **In production** and reconnect to avoid the Testing limit; this does not require exposing the backup server publicly. Tokens can still be revoked or expire under Google/Workspace policies. On `invalid_grant`, open the cloud destination and reconnect. Web OAuth makes re-authorization convenient; it does not remove Google's expiry policies.

### Encryption methods

| | Public key (recommended) | Passphrase |
| --- | --- | --- |
| Algorithm | age X25519 + ChaCha20-Poly1305 | age scrypt + ChaCha20-Poly1305 |
| Stored on the server | Public key only | Passphrase (encrypted with `connections.key`) |
| If the server/cloud is compromised | Backups still **cannot** be decrypted | An attacker who owns the server can decrypt |
| Needed to restore | Private key (`AGE-SECRET-KEY-1…`) | Passphrase |

- Click **Generate new key pair** to create keys: the private key is shown **only once** (copy it or download the `.txt`); the server does not store it. You can also run `age-keygen -o key.txt` yourself and paste only the public key. Multiple public keys (one per line) can be entered for backup keys.
- Store the private key/passphrase **separately from the server and the cloud account** (password manager, safe…). Lose the key = lose the backups.
- Changing keys only applies to subsequent uploads; older files still need the old key.
- age authenticates every data chunk: a file tampered with on the cloud fails to decrypt instead of producing wrong data.

### Restore from the cloud

- **In the UI**: **Cloud storage → Browse & download**, select a file, choose the connection to attach it to and enter the private key (kept in RAM only until the job finishes). The decrypted copy appears in the backup list with the source "From cloud", then restore as usual. Works on a fresh server too (just add the cloud destination again).
- **Manually** (if the server is lost):

  ```bash
  rclone copy gdrive:db-backups/Production/app_db/app_db_20261102T020000Z_1a2b3c4d.dump.age .
  age -d -i key.txt app_db_20261102T020000Z_1a2b3c4d.dump.age > app_db.dump
  # passphrase: age -d app_db_...dump.age > app_db.dump
  pg_restore --list app_db.dump
  createdb app_db_restored && pg_restore --no-owner -d app_db_restored app_db.dump
  ```

## Notifications, restore testing and monitoring

### Notifications

Admins go to **Notifications** (Account menu) → **Add channel**, choose a type, fill in the configuration and click **Send test** before saving. Each channel has its own events and message language (Tiếng Việt, English, 日本語):

| Event | When it is sent |
|---|---|
| Job failed | Backup, scheduled backup, restore, restore test, cloud upload/download or cleanup failed. Backups and cloud uploads only notify after the final retry. |
| Schedule missed | An enabled schedule is 15 minutes past its run time with no job created (stuck scheduler, stopped container…). Each missed run is reported once; failed jobs already have their own "failed" notification. |
| No recent backup | An enabled schedule has had no successful backup for longer than `BACKUP_MAX_AGE` (default `24h`; or one schedule interval + `JOB_TIMEOUT` if longer, e.g. ~26h for daily, ~7 days for weekly). Also catches jobs that keep running but keep failing. Reported once until a backup succeeds. |
| Backup unusually small | A new backup is smaller than the previous one of the same database by more than `BACKUP_SHRINK_PERCENT` (default `50`%). Skipped when the previous backup is under 1 MiB. |
| Connection lost | Every 5 minutes, each connection is checked (like the *Test connection* button). Reported after 2 consecutive failures (~10 minutes), and once more when it recovers. Webhooks receive `event` `connection_down` / `connection_up`. |
| Cloud unreachable | Every 30 minutes, a small file is written/read on each cloud destination. Reported after 2 consecutive failures and on recovery (`destination_down` / `destination_up`). |
| Disk almost full | Free space on the `BACKUP_DIR` volume is below `DISK_ALERT_PERCENT` (default `10`%), checked every 10 minutes; reported again once free space is above the threshold + 2% (`disk_low` / `disk_ok`). |
| Restore requested | As soon as someone requests a restore, including who, which backup and the target database. |
| Repeated failed logins | An IP address fails to sign in 10 times and is locked out for 15 minutes. Behind a reverse proxy, all requests may carry the proxy's IP. |
| Job completed | Successful backups, restores and restore tests. Can be noisy. |

| Type | Configuration |
|---|---|
| Telegram | Bot token from @BotFather and Chat ID (groups look like `-100…`) |
| Slack / Discord | Incoming webhook URL (`https://…`) |
| Webhook | URL receiving a JSON POST (`event`, `title`, `text`, `connection`, `database`, `job`…). Optional secret: header `X-DBBM-Signature: sha256=<HMAC-SHA256 of the body>` |
| Email | SMTP server, STARTTLS (587) / SSL (465) / unencrypted, optional account, sender, up to 20 recipients |

Tokens, webhook URLs and SMTP passwords are encrypted with AES-GCM like connection passwords and are never returned by the API; leave them empty when editing to keep them. Delivery failures are recorded in the activity log (`notifier.failed`) and the `dbbm_notification_failures_total` metric.

### Retries and catch-up

| Variable | Default | Meaning |
|---|---|---|
| `JOB_RETRIES` | `2` | Number of retries for failed backups / cloud uploads (0–10). While waiting, the job is "Queued" with the previous error. |
| `JOB_RETRY_DELAY` | `2m` | Delay before the first retry, doubled after each attempt. |
| `DISK_ALERT_PERCENT` | `10` | Send "Disk almost full" when free space falls below this percentage. `0` = off. |
| `BACKUP_SHRINK_PERCENT` | `50` | Send "Backup unusually small" when a new backup shrinks by more than this percentage. `0` = off. |
| `BACKUP_MAX_AGE` | `24h` | Send "No recent backup" when there has been no successful backup for this long. `0` = off. |
| `CATCH_UP_MISSED` | `true` | On startup, each schedule that missed a run while stopped runs once (several missed runs still produce a single backup). |

A restart while a job is waiting to retry marks it as failed, like other queued jobs.

### Restore testing

A restore test proves a backup **can be restored**, which checksums and `pg_restore --list` cannot. Steps: verify the checksum → restore into a new database named `dbbm_verify_<random>` using the restore account → open the database and count tables/collections → **drop the temporary database** (even if the restore fails or hits `JOB_TIMEOUT`). The manager checks that the temporary name doesn't exist before creating it, so it never drops a pre-existing database.

- **Manual**: the flask button on each backup; choose a test server (same database type, restore enabled).
- **Automatic**: when creating a schedule, enable *Test restore after every backup* and choose a test server.
- Results appear next to the backup (*Restore OK* / *Restore failed*, hover for details) and are sent to notification channels.

A restore test uses the test server's CPU, RAM and disk, and needs free space equal to the restored database. For large databases, point it at a staging server rather than production. Extra permissions for the restore account: PostgreSQL can drop databases it owns; MySQL needs `DROP`; MongoDB the `restore` role (without `dropDatabase`, the manager drops each collection); SQL Server `dbcreator`. On SQL Server, if the restore account cannot read the restored database, the result only confirms the database is ONLINE. If dropping the temporary database fails, the job reports an error with its name so you can drop it manually.

### Prometheus

Set `METRICS_TOKEN` (at least 16 characters) to enable `GET /metrics`; when unset, the endpoint returns 404. Scrape config:

```yaml
scrape_configs:
  - job_name: db-backup-manager
    authorization:
      credentials: "<METRICS_TOKEN>"
    static_configs:
      - targets: ["db-backup-manager:8080"]
```

| Metric | Labels | Meaning |
|---|---|---|
| `dbbm_backup_last_success_timestamp_seconds` | connection, database | When the last successful backup job finished |
| `dbbm_backup_last_failure_timestamp_seconds` | connection, database | When the last failed backup job finished |
| `dbbm_backup_last_duration_seconds` | connection, database | Duration of the last successful backup |
| `dbbm_backup_last_timestamp_seconds`, `dbbm_backup_last_size_bytes`, `dbbm_backups`, `dbbm_backups_bytes` | connection, database | Stored backups |
| `dbbm_verify_last_timestamp_seconds`, `dbbm_verify_last_success` | connection, database | Result of the last restore test |
| `dbbm_schedule_stale` | connection, database | 1 when a schedule missed its run without creating a job |
| `dbbm_destination_up` | destination | 1 when the last cloud destination check (every 30 minutes) succeeded, 0 on failure |
| `dbbm_connection_up` | connection | 1 when the last connection check (every 5 minutes) succeeded, 0 on failure |
| `dbbm_jobs` | kind, status | Number of jobs in history; `dbbm_queue_length` queued jobs |
| `dbbm_offsite_last_upload_timestamp_seconds` | destination | Last cloud upload |
| `dbbm_backup_disk_free_bytes`, `dbbm_backup_disk_size_bytes` | | Disk space of `BACKUP_DIR` |
| `dbbm_notification_failures_total` | | Failed notifications since startup |

Example alerts:

```yaml
groups:
  - name: db-backup
    rules:
      - alert: BackupTooOld
        expr: time() - dbbm_backup_last_success_timestamp_seconds > 26 * 3600
      - alert: BackupFailing
        expr: dbbm_backup_last_failure_timestamp_seconds > dbbm_backup_last_success_timestamp_seconds
      - alert: RestoreTestFailed
        expr: dbbm_verify_last_success == 0
      - alert: BackupDiskLow
        expr: dbbm_backup_disk_free_bytes / dbbm_backup_disk_size_bytes < 0.1
```

## Operating procedure

1. Check that the dashboard shows Connected and lists the databases you need to back up.
2. Run a manual backup and watch the job in History; only `completed` jobs produce a backup that can be downloaded/restored.
3. Create a schedule, e.g. `0 2 * * *` (daily at 02:00), with 14-day retention.
4. Add at least one notification channel and enable automatic restore testing for important schedules. Automatic tests confirm a backup can be restored; you should still periodically restore an archive into a new database and check business data from the application (rows, extensions, permissions).
5. Enable a cloud destination for backup schedules (see Cloud storage); periodically try downloading and decrypting a copy from the cloud. Metadata (`DATA_DIR`) is not uploaded automatically: back it up separately. To copy SQLite safely, stop the manager before copying the whole `DATA_DIR`, or use the SQLite online backup API; don't copy only `metadata.db` while WAL is active.
6. Archives are only listed when they have metadata. Do not add/rename files under `BACKUP_DIR` by hand; use Upload to import archives. A crash may leave `.partial` files or archives without metadata; only clean up orphaned files while the manager is stopped and after checking the metadata.

A restore that fails after the database was created leaves the target database in place for the DBA to inspect. Databases are never dropped automatically (except that restore tests always drop the temporary `dbbm_verify_…` database they created). To retry, use a new name or have the DBA drop the failed database after verification.

## Development and testing

Requires Go >= 1.26, Node >= 22 and the client tools for the database types you use in `PATH`: `pg_dump`/`pg_restore`, `mariadb-dump`/`mariadb`, `mongodump`/`mongorestore`. SQL Server needs no client tools.

```bash
cd web
npm ci
npm run build
cd ..
# Export the required variables; the local binary does not load .env.
export ADMIN_PASSWORD='SET_A_STRONG_LOCAL_PASSWORD'
make run
```

After signing in, add a local database connection under **Databases → Add connection**. The backend listens on `127.0.0.1:8080` by default. The built UI is served at http://127.0.0.1:8080. For React development, run `cd web && npm run dev`; Vite proxies `/api` to the backend.

```bash
make test
make build
```

Opt-in integration tests with a throwaway PostgreSQL, never production:

```bash
docker run -d --name dbbackup-test -e POSTGRES_PASSWORD=integration-only-password \
  -p 127.0.0.1:55439:5432 postgres:18-bookworm
# Wait: docker exec dbbackup-test pg_isready -U postgres
TEST_PGPORT=55439 go test -race -v ./...
docker rm -fv dbbackup-test
```

Or run them automatically with `sh scripts/integration.sh` (creates/removes the test container and picks a free port).

Tests create randomly named databases and clean up after themselves; they check real data after restore, rejection of existing targets, checksums, upload/download, deletion and scheduler callbacks. Unit tests cover viewer permissions, sessions/CSRF, restore confirmation, cron, retention, pgpass and job state recovery after restart.

## Limitations

- Manages multiple connections, but runs a single manager replica per metadata/backup directory pair. Database names for jobs must start with a letter or `_`, followed by letters/digits/`_`/`-`, up to 63 characters.
- The scheduler uses the timezone chosen by the admin in the UI (defaulting to `TZ`); changes apply immediately without a restart. The UI shows timestamps in the browser's timezone, labeled with the scheduler timezone.
- No support yet for PITR/WAL, volume snapshots, incremental file backups, overwrite restores or managing globals/roles through the UI. Upload directory backups are supported via Files connections.
- `pg_dump` saves a single database. Roles/tablespaces must be backed up separately with `pg_dumpall --globals-only`, stored and protected separately since they may contain sensitive information. Logical dumps do not provide point-in-time recovery (PITR). See [PostgreSQL pg_dump](https://www.postgresql.org/docs/current/app-pgdump.html).
- `pg_restore --list` checks the archive's table of contents and SHA-256 detects changes since creation/upload; neither proves data correctness or source trustworthiness. Restoring an archive may execute SQL controlled by its source; only use trusted archives. See [PostgreSQL pg_restore](https://www.postgresql.org/docs/current/app-pgrestore.html).
- Client and server versions need not match, but the dump client must not be older than the server, and archives from a newer client may not be readable by an older one. Restoring to an older server requires separate DBA assessment.

## Contributing

Issues and pull requests for bug fixes, documentation, translations or new features are welcome. For larger changes, open an issue first to discuss the scope and approach before writing code.

### Bug reports and feature requests

When reporting a bug, include the version or commit, deployment method, database type/version, steps to reproduce, expected result and logs with private information redacted. For feature requests, describe the use case and the desired behavior.

### Submitting a pull request

1. Fork the repository and create a branch for your change.
2. Keep changes focused; add documentation and tests that match the changed behavior.
3. Run `make test`. If you change the frontend, run `cd web && npm ci && npm run build`. If you change PostgreSQL backup/restore, also run `sh scripts/integration.sh` with Docker and the PostgreSQL client in `PATH`.
4. Open a pull request describing the problem, the change and how it was tested. Call out anything not tested against a real database.

Do not put passwords, tokens, encryption keys, metadata, backup data or private infrastructure addresses in issues, pull requests or test fixtures. Use the `example.com` domain and fake data for examples.

### Repository layout

```text
cmd/server/          Server entry point and CLI
internal/manager/    API, connections, backup/restore, scheduling, cloud and tests
web/                 React + TypeScript UI
scripts/             Docker image build/push and PostgreSQL integration tests
deploy/              Compose, environment template and Portainer guide
Dockerfile           Builds the frontend, backend and client tools
docker-compose.yml   Runs the app from source
.env.example         Configuration template for Docker Compose
Makefile             Development and test commands
```

## Security and private data

The repository only ships sample configuration. Supply accounts, passwords, OAuth settings and the Docker network yourself via the environment or a local `.env` file; enter database, cloud and notification details in the UI. When pushing/deploying the image, provide your own repository via `IMAGE`; no private registry is preconfigured.

The `.gitignore` and `.dockerignore` rules exclude local environment files, private keys/certificates, rclone configs, AWS/SSH/Docker credentials, metadata and backup archives at their default locations. When customizing paths or adding config files, check them before committing. Files already tracked by Git are not removed by adding ignore rules.

Connection passwords and cloud configs are encrypted with the `connections.key` file next to the metadata. Back up both the metadata and this key to be able to restore your configuration. Local backups are not age-encrypted automatically; age encryption applies before upload to the cloud. Protect access to the server, the data directory and the decryption keys.

Only restore archives from trusted sources. Checksum and format checks do not replace verifying data after a restore. Read the [operating procedure](#operating-procedure) and [limitations](#limitations) before using this for important data.

If a secret has been exposed, revoke or rotate it; removing it from the current version does not remove it from Git history. When reporting a security issue, do not post secrets, real data or sensitive exploit details in a public issue; use the hosting platform's private reporting channel if the repository has it enabled.

## License

This project is released under the [MIT](LICENSE) license.
