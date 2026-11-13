import type vi from "./vi";

const en: Record<keyof typeof vi, string> = {
  // common
  "common.loading": "Loading…",
  "common.cancel": "Cancel",
  "common.close": "Close",
  "common.edit": "Edit",
  "common.delete": "Delete",
  "common.change": "Change",
  "common.retry": "Retry",
  "pager.range": "{from}–{to} of {total}",
  "pager.page": "Page {n} of {total}",
  "pager.prev": "Previous page",
  "pager.next": "Next page",
  "common.viewAll": "View all",
  "common.none": "None yet",
  "common.no": "None",
  "common.menu": "Menu",
  "common.version": "Version",
  "common.databases": "Databases",
  "common.database": "Database",
  "common.backups": "Backups",
  "common.backup": "Backup",
  "common.restore": "Restore",
  "common.connection": "Connection",
  "common.allConnections": "All connections",
  "common.allDatabases": "All databases",
  "common.lastBackup": "Last backup",
  "common.schedules": "Schedules",
  "common.host": "Host",
  "common.account": "Account",
  "common.size": "Size",
  "common.time": "Time",
  "common.status": "Status",
  "common.active": "Active",
  "common.notConfigured": "Not configured",
  "common.keepBlank": "Leave blank to keep current",
  "common.optional": "optional",
  "common.saveChanges": "Save changes",
  "common.language": "Language",
  "error.request": "The request could not be completed",

  // time
  "time.justNow": "just now",
  "time.minutesAgo": "{n} {n|minute|minutes} ago",
  "time.hoursAgo": "{n} {n|hour|hours} ago",
  "time.daysAgo": "{n} {n|day|days} ago",

  // job status / kinds / backup sources
  "status.completed": "Completed",
  "status.running": "Running",
  "status.queued": "Queued",
  "status.failed": "Failed",
  "job.offsite": "Upload to cloud",
  "job.offsite_pull": "Download from cloud",
  "job.restore": "Restore",
  "job.retention": "Retention cleanup",
  "job.scheduled": "Scheduled backup",
  "job.backup": "Backup",
  "source.scheduled": "Scheduled",
  "source.upload": "Upload",
  "source.cloud": "From cloud",
  "source.manual": "Manual",

  // navigation
  "nav.overview": "Overview",
  "nav.databases": "Databases",
  "nav.backups": "Backups",
  "nav.schedules": "Schedules",
  "nav.activity": "Activity",
  "nav.settings": "Settings",
  "nav.connections": "Connections",
  "nav.allBackups": "All backups",
  "nav.cloud": "Cloud storage",
  "nav.activityLog": "Activity log",
  "nav.users": "Users",
  "nav.account": "Account",
  "nav.help": "Help",
  "help.subtitle":
    "How to use DB Backup Manager: connections, backups, restore, schedules, cloud and permissions.",
  "help.contents": "Contents",
  "topbar.jobsRunning": "{n} {n|job|jobs} running",

  // login
  "login.title": "Sign in",
  "login.subtitle": "Manage backups for your database servers.",
  "login.username": "Username",
  "login.password": "Password",
  "login.submit": "Sign in",

  // actions
  "action.backupNow": "Back up now",
  "action.addSchedule": "Add backup schedule",
  "action.recheck": "Check again",
  "action.createBackup": "Create backup",
  "action.uploadBackup": "Upload backup",
  "action.upload": "Upload",
  "action.download": "Download",
  "menu.open": "Open",
  "menu.actions": "Actions",

  // connections
  "conn.add": "Add connection",
  "conn.edit": "Edit connection",
  "conn.delete": "Delete connection",
  "conn.deleted": "Deleted connection",
  "conn.env": "Connection from env",
  "conn.deleteConfirm":
    "Delete connection <b>{name}</b>? Stored backups are kept. Remove related schedules and wait for running jobs to finish before deleting the connection.",
  "home.subtitle": "The database servers whose backups you manage.",
  "home.search": "Search by name or host…",
  "home.filterAll": "All ({n})",
  "home.filterOk": "Online ({n})",
  "home.filterError": "Errors ({n})",
  "home.schedulesOn": "{n} enabled",
  "home.noResults": "No results",
  "home.noResultsText": "Try a different keyword or filter.",
  "home.empty": "No connections yet",
  "home.emptyText": "Add your first database server to start backing up.",
  "probe.checking": "Checking",
  "probe.down": "Unreachable",

  // backups
  "backups.search": "Search databases…",
  "backups.count": "{n} {n|backup|backups} · {size}",
  "backups.restoreHint":
    "To restore, open the target connection's Settings and configure a restore account.",
  "backups.noMatch": "No matching backups",
  "backups.empty": "No backups yet",
  "backups.emptyText": "Create a backup or upload an existing backup file.",
  "backups.emptyNoAccess": "You have no backups within your granted access.",
  "backups.subtitle": "Backups of every connection in this account.",
  "backups.source": "Source",
  "backups.cloudCopies": "Encrypted copies on: {list}",
  "backups.toCloud": "Encrypt and upload to cloud",
  "backups.restore": "Restore to a new database",
  "backups.restoreDisabled":
    "Configure a restore account in the connection's Settings",
  "backups.delete": "Delete backup",

  // schedules
  "schedules.tzNotice":
    "Schedules run in <b>{tz}</b>. Retention only deletes scheduled backups; manual and uploaded backups are kept.",
  "schedules.empty": "No backup schedules yet",
  "schedules.emptyText": "Add a backup schedule and retention period.",
  "schedules.subtitle": "Backup schedules of every connection.",
  "schedules.nextRun": "Next run",
  "schedules.retention": "Retention",
  "schedules.days": "{n} {n|day|days}",
  "schedules.enabled": "Enabled",
  "schedules.paused": "Paused",
  "schedules.pause": "Pause",
  "schedules.enable": "Enable schedule",
  "schedules.edit": "Edit schedule",
  "schedules.delete": "Delete schedule",

  // jobs & audit
  "jobs.empty": "No activity yet",
  "jobs.emptyText": "Backup, restore and retention jobs will be recorded here.",
  "jobs.started": "Started",
  "jobs.finished": "Finished",
  "jobs.errorDetail": "Error details",
  "activity.subtitle": "Last 200 jobs · refreshes every 5 seconds",
  "activity.jobs": "Jobs",
  "activity.audit": "Audit log",
  "activity.auditAdmin":
    "Sign-ins, downloads, uploads, deletions, schedule changes · last 100 events",
  "activity.auditUser": "Your actions · last 100 events",
  "activity.auditEmpty": "No events yet",
  "activity.auditEmptyText": "Your actions will be recorded here.",
  "audit.event": "Event",
  "audit.actor": "Actor",
  "audit.detail": "Details",
  "audit.system": "System",

  // connection overview
  "overview.dataSize": "{size} of data",
  "overview.notConnected": "Not connected",
  "overview.backups": "Backups",
  "overview.stored": "{size} stored",
  "overview.activeSchedules": "Active schedules",
  "overview.scheduleCount": "{n} {n|schedule|schedules} · {tz}",
  "overview.recentBackups": "Recent backups",
  "overview.backupsEmptyText": "Backups will appear here once completed.",
  "overview.recentActivity": "Recent activity",
  "overview.quickActions": "Quick actions",
  "overview.connInfo": "Connection details",
  "overview.connSettings": "Connection settings",
  "dbs.loadError": "Could not load the database list",
  "dbs.connecting": "Connecting…",
  "dbs.checking": "Checking the connection to the database server.",
  "dbs.empty": "No databases",
  "dbs.emptyText": "This account cannot see any databases on the server.",
  "zone.notFound": "Connection not found",
  "zone.notFoundText": "This connection may have been deleted.",
  "zone.back": "Back to connections",
  "zone.pgError": "Cannot connect to the database server.",

  // connection settings
  "settings.server": "Server",
  "settings.serverText":
    "{addr} · user <b>{user}</b> · database <b>{db}</b> · SSL {ssl}",
  "settings.hasPassword": "password set",
  "settings.noPassword": "no password",
  "settings.restoreOn":
    "Enabled with account <b>{user}</b>. Restore always creates a new database on this server.",
  "settings.restoreOff":
    "No restore account configured. Backups from other connections cannot be restored to this server.",
  "settings.configure": "Configure",
  "settings.check": "Test connection",
  "settings.currentStatus": "Current status:",
  "settings.deleteText":
    "Stored backups are kept. Remove related schedules and wait for running jobs to finish before deleting.",

  // cloud
  "cloud.subtitle":
    "Backups are encrypted on the server before being uploaded to Google Drive, OneDrive, S3…",
  "cloud.noRclone":
    "<code>rclone</code> was not found on the backup server. Rebuild the Docker image or install rclone to use cloud storage.",
  "cloud.encryption": "Encryption",
  "cloud.copies": "Copies",
  "cloud.latest": "Latest",
  "cloud.keep": "Keep",
  "cloud.keepDays": "{n} {n|day|days} (latest copy always kept)",
  "cloud.forever": "Forever",
  "cloud.scheduleUse": "{n} {n|schedule uses|schedules use} this destination",
  "cloud.browse": "Browse & download",
  "cloud.empty": "No cloud destinations yet",
  "cloud.emptyText":
    "Add Google Drive, OneDrive, S3… to automatically send encrypted backups off the server.",
  "cloud.decryptNote":
    "Manual decryption if the server is lost: <code>age -d -i key.txt file.dump.age > file.dump</code> (or <code>age -d file.dump.age</code> and enter the passphrase), then restore with the database's tool (<code>pg_restore</code>, <code>mariadb</code>, <code>mongorestore</code> or <code>RESTORE DATABASE</code>). Cloud retention runs with the daily cleanup at 03:17.",
  "dest.add": "Add cloud destination",
  "dest.edit": "Edit cloud destination",
  "dest.delete": "Delete cloud destination",
  "dest.deleted": "Deleted destination",
  "dest.deleteConfirm":
    "Delete destination <b>{name}</b>? Uploaded files stay in the cloud, but the app will no longer manage or clean them up. Remove the destination from backup schedules first.",

  // users
  "users.adminRequired": "Administrator access required",
  "users.adminRequiredText": "Only administrators can manage users.",
  "users.subtitle":
    "Sign-in accounts and permissions per connection and database.",
  "users.add": "Add user",
  "users.edit": "Edit user",
  "users.delete": "Delete user",
  "users.user": "User",
  "users.role": "Role",
  "users.permissions": "Permissions",
  "users.createdAt": "Created",
  "users.fullAccess": "Full access",
  "users.locked": "Locked",
  "users.empty": "No users yet",
  "users.emptyText":
    "Create accounts for teammates and grant access per connection or per database.",
  "users.deleteConfirm":
    "Delete user <b>{name}</b>? The account is signed out immediately and loses all permissions. Backups and schedules are kept.",
  "role.admin": "Administrator",
  "role.viewer": "Read-only",
  "role.user": "User",
  "level.view": "View",
  "level.operate": "Operate",
  "grants.none": "No permissions granted",
  "grants.allDbs": "all DBs",

  // user menu & profile
  "menu.timezone": "Timezone",
  "menu.profile": "Profile & password",
  "menu.changeTimezone": "Change schedule timezone",
  "menu.users": "Manage users",
  "menu.logout": "Sign out",
  "profile.title": "Profile",
  "profile.changePassword": "Change password",
  "profile.viewerNote":
    "Viewer account: change the password via <code>VIEWER_PASSWORD</code> in the <code>.env</code> file, then restart the service.",
  "pw.current": "Current password",
  "pw.new": "New password",
  "pw.hint": "12–72 characters. Other sessions will be signed out.",
  "pw.confirm": "Confirm new password",
  "pw.mismatch": "Passwords do not match",

  // timezone modal
  "tz.title": "Schedule timezone",
  "tz.label": "Timezone (IANA)",
  "tz.default": "Default from <code>TZ</code> in .env: {tz}",
  "tz.note":
    "All backup schedules and the 03:17 cleanup will use the new timezone right after saving. Cron expressions stay the same; only how the time is interpreted changes.",
  "tz.reset": "Reset to default",
  "tz.save": "Save timezone",

  // modals: shared pieces
  "modal.chooseDb": "Choose a database",
  "modal.noUpload": "Don't upload",
  "modal.destHint":
    "Encrypted with age and uploaded once the backup completes.",
  "modal.destHintEmpty":
    "Add a destination under Cloud storage to send backups off the server.",
  "modal.cloudFiles": "Files on {name}",

  // backup / restore / delete
  "backup.intro":
    "Creates a full backup of the database and verifies its SHA-256 checksum when done.",
  "backup.start": "Start backup",
  "backup.delete": "Delete backup",
  "backup.deleteConfirm":
    "Permanently delete the backup of <b>{db}</b> ({date}, {size}) from the server?",
  "restore.title": "Restore database",
  "restore.notice":
    "Restore always creates a <b>new database</b>. The target name must not already exist.",
  "restore.target": "Target connection",
  "restore.notConfigured": "Restore not configured",
  "restore.account": "Restore account: {user}",
  "restore.targetName": "Target database name",
  "restore.confirmName": "Type the target name again to confirm",
  "restore.trust":
    "I trust the source of this archive and agree to execute its SQL content during restore.",
  "restore.submit": "Confirm restore",

  // schedule modal
  "schedule.daily": "Daily · 02:00",
  "schedule.every6h": "Every 6 hours",
  "schedule.sunday": "Sunday · 02:00",
  "schedule.cron": "Cron expression",
  "schedule.cronHint": "minute · hour · day · month · weekday — {tz}",
  "schedule.retention": "Retention (days)",
  "schedule.sendCloud": "Send a copy to cloud",
  "schedule.note":
    "If several schedules cover the same database, the longest retention applies. Expired backups are deleted after each run and daily at 03:17. Cloud copies use the cloud destination's own retention.",
  "schedule.save": "Save schedule",

  // upload modal
  "upload.title": "Upload backup",
  "upload.chooseFile": "Choose a backup file",
  "upload.intro":
    "Accepts {format}. Limit {size}. Only upload files from sources you trust.",
  "upload.dbName": "Original database name",
  "upload.pick": "Choose a file ({ext})",
  "upload.submit": "Upload & verify",

  // to-cloud modal
  "toCloud.dest": "Cloud destination",
  "toCloud.exists": "(already there — upload again)",
  "toCloud.note":
    "The archive is checked with SHA-256 and encrypted with age on the server before upload; the cloud only receives encrypted files (.dump.age).",
  "toCloud.submit": "Encrypt & upload",

  // cloud files / pull modal
  "pull.intro":
    "Downloads the encrypted copy to the server, decrypts it and verifies the archive. The download shows up in the backup list (source “{source}”) for restore.",
  "pull.loading": "Reading file list…",
  "pull.onServer": "already on server",
  "pull.empty": "No files yet",
  "pull.emptyText": "No .dump.age files in /{path}",
  "pull.dbName": "Original database",
  "pull.privateKey": "age private key",
  "pull.otherPassphrase": "Different passphrase (optional)",
  "pull.passphrasePlaceholder": "Leave blank to use the stored passphrase",
  "pull.secretHint":
    "Kept in memory only until the decrypt job finishes; never stored in the database.",
  "pull.submit": "Download & decrypt",

  // toasts
  "notice.loggedOut": "Signed out",
  "notice.scheduleUpdated": "Schedule updated",
  "notice.scheduleDeleted": "Backup schedule deleted",
  "notice.scheduleCreated": "Backup schedule created",
  "notice.tzChanged": "Schedule timezone changed",
  "notice.passwordChanged": "Password changed",
  "notice.userSaved": "User saved",
  "notice.userCreated": "User created",
  "notice.userDeleted": "User deleted",
  "notice.connSaved": "Connection saved",
  "notice.connDeleted": "Connection deleted",
  "notice.backupQueued": "Backup queued",
  "notice.restoreQueued": "Restore queued",
  "notice.uploaded": "Archive uploaded and verified",
  "notice.destSaved": "Cloud destination saved",
  "notice.destDeleted": "Cloud destination deleted",
  "notice.backupDeleted": "Backup deleted",
  "notice.offsiteQueued": "Cloud upload queued",
  "notice.pullQueued": "Download job queued",

  // connection form
  "connForm.testOkRestore":
    "The backup connection and the restore account's privileges are valid",
  "connForm.testOk": "Connected to {engine} successfully",
  "connForm.name": "Connection name",
  "connForm.namePlaceholder": "e.g. Production, Staging",
  "connForm.hostPlaceholder": "db.example.com or IP",
  "connForm.clearPassword":
    "Remove the stored password (when the password field is blank)",
  "connForm.database": "Connection database",
  "connForm.databaseHint": "Used to list databases. Default: postgres.",
  "connForm.restoreUser": "Restore account",
  "connForm.restorePassword": "Restore password",
  "connForm.restoreHint":
    "Leave blank to disable restore. Requires LOGIN, CREATEDB and permission to connect to the maintenance database on this server. Restore only creates new databases; objects are owned by the restore account.",
  "connForm.clearRestorePassword":
    "Remove the stored restore password (when the password field is blank)",
  "connForm.hostNote":
    "The host must be reachable from the backup server; localhost means that server or container itself.",

  // rclone provider help
  "help.other": "Other",
  "help.drive1":
    "On a machine with a browser, install <rclone>rclone</rclone> and run <code>rclone config</code> → <code>n</code> (new remote) → choose <code>drive</code>.",
  "help.drive2":
    "Pick the <code>drive.file</code> scope (rclone only sees files it created). Sign in to Google when the browser opens.",
  "help.drive3":
    "Run <code>rclone config show <remote-name></code> and paste the whole output below.",
  "help.onedrive1":
    "On a machine with a browser, run <code>rclone config</code> → <code>n</code> → choose <code>onedrive</code> and sign in to Microsoft.",
  "help.onedrive2":
    "Choose the OneDrive type (Personal / Business / SharePoint) and drive when rclone asks.",
  "help.onedrive3":
    "Run <code>rclone config show <remote-name></code> and paste the whole output below. The token is refreshed automatically and saved after each run.",
  "help.s31":
    "Create an access key limited to read/write/delete on the backup bucket.",
  "help.s32":
    "Fill in the config following the template (provider: <code>AWS</code>, <code>Cloudflare</code>, <code>Minio</code>, <code>Wasabi</code>…). The folder below starts with the bucket name.",
  "help.otherText":
    "Any <link>rclone backend</link> is supported (Dropbox, Box, B2, SFTP, WebDAV, pCloud…). Create a remote with <code>rclone config</code>, then paste the output of <code>rclone config show <remote-name></code>.",

  // destination form
  "keyfile.title": 'Private key to decrypt backups for destination "{name}"',
  "keyfile.decrypt": "Decrypt",
  "destForm.passMismatch": "Passphrases do not match",
  "destForm.confirmKeySaved":
    "Confirm that you saved the private key before saving the destination",
  "destForm.testOk":
    "Wrote, read and deleted a test file on the cloud successfully",
  "destForm.storage": "Storage",
  "destForm.name": "Name",
  "destForm.namePlaceholder": "e.g. Company Google Drive",
  "destForm.config": "rclone config",
  "destForm.configKeep": "Leave blank to keep the stored {provider} config",
  "destForm.configHint":
    "Encrypted with the server key before saving; never sent back to the browser.",
  "destForm.remotePath": "Cloud folder",
  "destForm.remotePathHint":
    "Files are stored at <folder>/<connection>/<database>/",
  "destForm.keepDays": "Keep (days)",
  "destForm.forever": "0 = forever",
  "destForm.publicKeyRec": "Public key (recommended)",
  "destForm.passphrase": "Passphrase",
  "destForm.recipientNote":
    "Backups are encrypted with <b>age</b> (X25519 + ChaCha20-Poly1305). The server only holds the public key, so <b>even if the server or cloud account is compromised, backup files stay unreadable</b>. The private key is needed to download and decrypt.",
  "destForm.publicKey": "age public key",
  "destForm.publicKeyPlaceholder":
    "age1… (one key per line; you can add backup keys)",
  "destForm.genKey": "Generate new key pair",
  "destForm.privateOnce": "Private key — shown only once",
  "destForm.copy": "Copy",
  "destForm.downloadTxt": "Download .txt",
  "destForm.privateHint":
    "The server does not store the private key. Keep it somewhere safe, separate from the server and cloud account (e.g. a password manager). Losing the private key = backups cannot be recovered.",
  "destForm.keySaved": "I have saved the private key somewhere safe.",
  "destForm.passphraseNote":
    "age encryption with a passphrase (scrypt). The passphrase is stored encrypted on the server for automatic uploads — anyone who takes over the server can decrypt too. Keep the passphrase elsewhere so you can recover if the server is lost.",
  "destForm.minChars": "At least 16 characters",
  "destForm.repeat": "Repeat",
  "destForm.rekeyHint":
    "Key changes only apply to future uploads; existing cloud files still need the old key to decrypt.",
  "destForm.test": "Test",

  // user form
  "userForm.displayName": "Display name",
  "userForm.displayPlaceholder": "e.g. Jane Doe",
  "userForm.resetPassword": "Reset password",
  "userForm.pwLength": "12–72 characters",
  "userForm.resetHint": "Resetting the password signs this user out.",
  "userForm.userText":
    "Sees and operates only on granted connections and databases.",
  "userForm.adminText":
    "Full access: connections, cloud, users and every database.",
  "userForm.lock": "Lock account (cannot sign in)",
  "userForm.grants": "Permissions",
  "userForm.level": "Access level",
  "userForm.removeGrant": "Remove permission",
  "userForm.addGrant": "Add permission",
  "userForm.create": "Create user",

  // database engines
  "connForm.engine": "Database type",
  "connForm.authDatabase": "Authentication database",
  "connForm.databaseHintMongo":
    "Database holding the user (authSource). Default: admin.",
  "connForm.databaseHintMssql":
    "Used to sign in and list databases. Default: master.",
  "connForm.mongoNoAuth": "Leave blank if MongoDB has access control disabled.",
  "connForm.sharedDir": "Shared backup directory",
  "connForm.serverDir": "Path on SQL Server",
  "connForm.localDir": "Path in this container",
  "connForm.sharedDirHint":
    "SQL Server writes .bak files here with BACKUP DATABASE. The backup container must mount the same directory (shared volume, SMB or NFS) read-write, and SQL Server must be able to read files the container writes.",
  "connForm.restoreHintMysql":
    "Leave blank to disable restore. Requires the global CREATE privilege and full rights on newly created databases. Restore only creates new databases; DEFINER clauses are removed so objects belong to the restore account.",
  "connForm.restoreHintMongo":
    "Leave blank to disable restore. Requires the restore role (or readWrite on the target) and listDatabases. Restore only writes to databases that do not exist yet.",
  "connForm.restoreHintMssql":
    "Leave blank to disable restore. Requires dbcreator (CREATE ANY DATABASE). Restore only creates new databases; data files go to SQL Server's default folders.",
  "restore.noTarget": "No {engine} connection has restore enabled",
  // restore tests, retries and notifications
  "job.verify": "Restore test",
  "nav.notifications": "Notifications",
  "jobs.attempt": "Attempt {n}",
  "verify.action": "Test restore",
  "verify.title": "Test restore",
  "verify.passed": "Restore test passed",
  "verify.failed": "Restore test failed",
  "verify.badgeOk": "Restore OK",
  "verify.badgeFail": "Restore failed",
  "verify.scheduleBadge": "Tested",
  "verify.intro":
    "The backup is restored into a <b>new temporary database</b> (named <code>dbbm_verify_…</code>), checked to open, its tables counted, and then the <b>temporary database is dropped</b>. No other database is touched. While it runs it uses CPU, memory and disk space on the test server.",
  "verify.server": "Test server",
  "verify.serverHint":
    "Uses this server's restore login. Prefer a staging server for large databases.",
  "verify.submit": "Run test",
  "schedule.verify": "Test restore after every backup",
  "schedule.verifyHint":
    "Restores the new backup into a temporary database, checks it and drops it. The result is shown next to the backup and sent to notification channels.",
  "schedule.verifyUnavailable":
    "Needs a connection of the same database type with a restore login.",
  "schedule.verifyServer": "Test server",
  "schedule.verifyServerHint":
    "Tests create and drop a database on this server; prefer a staging server.",
  "schedule.sameServer": "same server",
  "schedule.verifyOn": "Restore tested after every backup on {server}",
  "notice.verifyQueued": "Restore test queued",
  "notice.notifierSaved": "Notification channel saved",
  "notice.notifierDeleted": "Notification channel deleted",
  "notify.subtitle":
    "Get told on Telegram, Slack, Discord, email or a webhook when a backup fails or a schedule does not run.",
  "notify.add": "Add channel",
  "notify.edit": "Edit channel",
  "notify.delete": "Delete channel",
  "notify.deleteConfirm":
    "Delete <b>{name}</b>? It will stop receiving notifications.",
  "notify.events": "Events",
  "notify.on": "On",
  "notify.off": "Off",
  "notify.urlHidden": "URL hidden",
  "notify.empty": "No notification channels",
  "notify.emptyText":
    "Add a channel to hear about failed backups right away instead of checking this page.",
  "notify.note":
    "Tokens, webhook URLs and SMTP passwords are encrypted in the metadata and never shown again. Failed jobs are <b>retried automatically</b> before you are told (see <code>JOB_RETRIES</code>, <code>JOB_RETRY_DELAY</code>).",
  "notify.event.failure": "Job failed",
  "notify.event.stale": "Schedule did not run",
  "notify.event.connection": "Database connection lost",
  "notify.event.overdue": "No backup for too long",
  "notify.event.backup_size": "Backup unusually small",
  "notify.event.destination": "Cloud unreachable",
  "notify.event.disk": "Disk almost full",
  "notify.event.restore": "Restore requested",
  "notify.event.security": "Too many failed logins",
  "notify.event.success": "Job completed",
  "notify.eventHint.failure":
    "A backup, restore, restore test, cloud upload or cleanup failed (after retries).",
  "notify.eventHint.stale":
    "A scheduled run was due but no job was created, e.g. while the container was down.",
  "notify.eventHint.connection":
    "Connections are checked every 5 minutes; you are told after 2 failed checks in a row and again when the connection is back.",
  "notify.eventHint.overdue":
    "An enabled schedule has had no successful backup for longer than BACKUP_MAX_AGE (default 24h), even if jobs keep running and failing.",
  "notify.eventHint.backup_size":
    "A new backup is more than BACKUP_SHRINK_PERCENT (default 50%) smaller than the previous one: data may be missing or deleted.",
  "notify.eventHint.destination":
    "Cloud destinations are checked every 30 minutes; you are told after 2 failed checks in a row and again when they work again.",
  "notify.eventHint.disk":
    "Free space of the backup folder drops below DISK_ALERT_PERCENT (default 10%), and when there is room again.",
  "notify.eventHint.restore":
    "As soon as someone requests a restore, with who did it and the target database.",
  "notify.eventHint.security":
    "An IP address failed to log in 10 times and is locked out for 15 minutes.",
  "notify.eventHint.success":
    "Successful backups, restores and restore tests. Can be noisy.",
  "notifyForm.name": "Channel name",
  "notifyForm.namePlaceholder": "e.g. Ops team",
  "notifyForm.botToken": "Bot token",
  "notifyForm.chatId": "Chat ID",
  "notifyForm.webhookUrl": "Webhook URL",
  "notifyForm.hmacSecret": "HMAC signing secret",
  "notifyForm.optional": "Optional",
  "notifyForm.smtpHost": "SMTP server",
  "notifyForm.smtpUser": "SMTP username",
  "notifyForm.smtpPassword": "SMTP password",
  "notifyForm.from": "From",
  "notifyForm.to": "To",
  "notifyForm.security": "Connection security",
  "notifyForm.noTls": "None (internal networks only)",
  "notifyForm.events": "Send when",
  "notifyForm.lang": "Message language",
  "notify.pause": "Pause",
  "notify.resume": "Resume",
  "notice.notifierPaused": "Notification channel paused",
  "notice.notifierResumed": "Notification channel resumed",
  "notifyForm.test": "Send test",
  "notifyForm.testOk": "Test message sent. Check the channel.",
  "notifyForm.help.telegram":
    "1. Talk to <b>@BotFather</b> and send <code>/newbot</code> to get a bot token. 2. Add the bot to a group (or message it once). 3. Find the chat ID at <code>https://api.telegram.org/bot<token>/getUpdates</code> under <code>chat.id</code> (groups look like <code>-100…</code>).",
  "notifyForm.help.slack":
    "In Slack: <b>Apps → Incoming Webhooks</b>, pick a channel and copy the webhook URL (<code>https://hooks.slack.com/services/…</code>).",
  "notifyForm.help.discord":
    "In Discord: <b>Channel settings → Integrations → Webhooks → New webhook</b>, copy the URL.",
  "notifyForm.help.webhook":
    "Sends a JSON POST with <code>event</code>, <code>title</code>, <code>text</code>, <code>connection</code>, <code>database</code> and <code>job</code>. With a secret, <code>X-DBBM-Signature: sha256=…</code> is the HMAC-SHA256 of the body. Works with Microsoft Teams Workflows, n8n, Zapier…",
  "notifyForm.help.email":
    "Gmail/Google Workspace: <code>smtp.gmail.com</code>, STARTTLS 587, with an <b>app password</b> rather than the account password. Separate several recipients with commas.",
  // branding
  "menu.branding": "Branding",
  "branding.title": "Name and logo",
  "branding.name": "Display name",
  "branding.nameHint":
    "Shown in the top bar, the sign-in page and the browser tab. Leave blank for “{name}”.",
  "branding.pick": "Choose a logo image (PNG, JPEG, WebP, GIF, SVG)",
  "branding.logoHint":
    "Up to 256 KB. A square image with a transparent background works best; it is also used as the favicon.",
  "branding.removeLogo": "Use the default logo",
  "branding.tooLarge": "The logo must be 256 KB or smaller",
  "notice.brandingSaved": "Name and logo saved",
  // files engine
  "connForm.sourceDir": "Source folder (in container)",
  "connForm.sourceDirHint":
    "The folder to back up, mounted into the container (read-only is recommended), e.g. /sources/uploads.",
  "connForm.itemName": "Backup item name",
  "connForm.itemNameHint":
    "Identifies this folder in the backup list, schedules, permissions and download file names (e.g. uploads-….tar.gz). Taken from the source folder name; keep it once backups exist.",
  "connForm.restoreDir": "Restore folder (in container)",
  "connForm.restoreDirHint":
    "Backups are extracted into a new subfolder inside this folder, never over existing files. Must be writable by UID 999. Leave blank to disable restore.",
  "connForm.filesNote":
    "The container can only read folders mounted into it. Add a volume in docker-compose, e.g. /var/www/app/uploads:/sources/uploads:ro",
  "connForm.testOkFiles": "Source folder is readable",
  "common.folder": "Folder",
  "restore.folder": "Extract into: {dir}",
  "restore.targetFolder": "Target folder name (new folder)",
  "settings.filesText": "Source folder {addr} · backup item <b>{db}</b>",
  "settings.restoreDir": "Restore into folder <b>{user}</b>",
  // Google Drive web OAuth
  "googleOAuth.option": "Sign in with Google OAuth",
  "googleOAuth.description":
    "Sign in to Google to authorize backup storage. Tokens stay on the server; no rclone installation is needed on your computer.",
  "googleOAuth.setup":
    "An administrator must configure GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET and GOOGLE_OAUTH_REDIRECT_URL on the server. See the Google OAuth instructions in README.",
  "googleOAuth.callback": "Registered Google callback URL:",
  "googleOAuth.tunnel":
    "Through an SSH tunnel: open the app at localhost using the port in the callback URL. The server does not need to be public.",
  "googleOAuth.ready":
    "Google Drive authorized. Save this destination within 30 minutes to keep the connection.",
  "googleOAuth.connect": "Connect Google Drive",
  "googleOAuth.reconnect": "Reconnect Google Drive",
  "googleOAuth.waiting": "Waiting for Google authorization…",
  "googleOAuth.popup": "Allow the sign-in popup in your browser and try again.",
  "googleOAuth.required": "Connect Google Drive before testing or saving.",
  "googleOAuth.scope": "Drive access",
  "googleOAuth.scopeFile": "Only files created by this app (recommended)",
  "googleOAuth.scopeFull": "Full Drive — existing folders or backups",
  "googleOAuth.scopeHint":
    "When switching OAuth clients and accessing older backups, select full Drive and reconnect. This permits the app to read and modify all Drive files.",
};

export default en;
