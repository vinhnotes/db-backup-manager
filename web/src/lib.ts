import { useEffect, useState } from "react";
import { Key, locale, t, translateServer } from "./i18n";

export type Backup = {
  connection_id: string;
  id: string;
  database: string;
  size: number;
  checksum: string;
  created_at: string;
  source: string;
  engine: Engine;
  /** Result of the latest restore test: "", "passed" or "failed". */
  verify_status?: string;
  verify_detail?: string;
  verified_at?: string;
};
export type Job = {
  connection_id: string;
  id: string;
  kind: string;
  database: string;
  backup_id: string;
  destination_id: string;
  remote_path: string;
  status: string;
  error: string;
  created_at: string;
  started_at?: string;
  finished_at: string;
  /** Automatic retries already made (0 = first try). */
  attempt?: number;
};
export type Schedule = {
  connection_id: string;
  id: number;
  database: string;
  cron: string;
  retention_days: number;
  enabled: boolean;
  destination_id: string;
  /** Run a restore test after each backup of this schedule. */
  verify?: boolean;
  /** Server for the restore test; "" = the schedule's own connection. */
  verify_connection_id?: string;
  next_run?: string;
  last_run_at?: string;
};
/** Product name and logo, set by an administrator. */
export type Branding = {
  name: string;
  default_name: string;
  logo: boolean;
  /** Changes on every update; appended to the logo URL. */
  version: string;
};
export const logoURL = (b: Branding) =>
  "/api/branding/logo?v=" + encodeURIComponent(b.version);
export type NotifierType =
  | "telegram"
  | "slack"
  | "discord"
  | "webhook"
  | "email";
export type NotifierEvent =
  | "failure"
  | "success"
  | "stale"
  | "connection"
  | "disk"
  | "backup_size"
  | "overdue"
  | "destination"
  | "restore"
  | "security";
/** A notification channel. Secret settings are never returned. */
export type Notifier = {
  id: string;
  name: string;
  type: NotifierType;
  config: Record<string, string>;
  events: NotifierEvent[];
  lang: "vi" | "en" | "ja";
  enabled: boolean;
  created_at: string;
  secrets_set: string[];
};
export type Destination = {
  id: string;
  name: string;
  type: string;
  has_config: boolean;
  remote_path: string;
  encryption: "recipient" | "passphrase";
  recipients: string;
  has_passphrase: boolean;
  retention_days: number;
  created_at: string;
};
/** An encrypted copy of a backup stored on a cloud destination. */
export type Offsite = {
  backup_id: string;
  destination_id: string;
  connection_id: string;
  database: string;
  remote_path: string;
  size: number;
  checksum: string;
  backup_created_at: string;
  uploaded_at: string;
};
export type RemoteFile = {
  path: string;
  size: number;
  mod_time: string;
  database: string;
  backup_id: string;
};
export type ConnectionProfile = {
  id: string;
  engine: Engine;
  /** SQL Server: shared backup directory as seen by SQL Server.
   * Files: the restore directory (optional). */
  server_dir: string;
  /** SQL Server: the same directory as mounted in this container.
   * Files: the source directory to back up. */
  local_dir: string;
  name: string;
  host: string;
  port: string;
  username: string;
  database: string;
  sslmode: string;
  has_password: boolean;
  restore_username: string;
  restore_enabled: boolean;
  has_restore_password: boolean;
};
export type Engine = "postgres" | "mysql" | "mongodb" | "mssql" | "files";
/** Per-engine defaults; mirrors the engines in internal/manager. */
export const engines: Record<
  Engine,
  {
    label: string;
    port: string;
    /** Maintenance/auth database; "" when the engine has none. */
    database: string;
    sslmode: string;
    sslModes: string[];
    /** Upload formats, for the file picker. */
    accept: string;
    format: string;
  }
> = {
  postgres: {
    label: "PostgreSQL",
    port: "5432",
    database: "postgres",
    sslmode: "prefer",
    sslModes: [
      "disable",
      "allow",
      "prefer",
      "require",
      "verify-ca",
      "verify-full",
    ],
    accept: ".dump,.backup",
    format: "pg_dump -Fc (.dump)",
  },
  mysql: {
    label: "MySQL/MariaDB",
    port: "3306",
    database: "",
    sslmode: "prefer",
    sslModes: ["disable", "prefer", "require", "verify-full"],
    accept: ".sql,.gz",
    format: "mysqldump / mariadb-dump (.sql, .sql.gz)",
  },
  mongodb: {
    label: "MongoDB",
    port: "27017",
    database: "admin",
    sslmode: "disable",
    sslModes: ["disable", "require", "verify-full"],
    accept: ".gz,.archive",
    format: "mongodump --archive --gzip",
  },
  mssql: {
    label: "SQL Server",
    port: "1433",
    database: "master",
    sslmode: "prefer",
    sslModes: ["disable", "prefer", "require", "verify-full"],
    accept: ".bak",
    format: "SQL Server BACKUP (.bak)",
  },
  files: {
    label: "Files",
    port: "",
    database: "files",
    sslmode: "",
    sslModes: [],
    accept: ".tar.gz,.tgz,.gz",
    format: "tar.gz",
  },
};
/** Default backup item name of a files connection: the folder's last path
 * part, made a valid name ("/sources/2024 uploads" → "_2024_uploads").
 * Mirrors filesItemName in internal/manager/engine_files.go. */
export function filesItemName(dir: string) {
  const base = dir.replace(/\/+$/, "").split("/").pop() || "";
  let name = base.replace(/[^a-zA-Z0-9_-]/g, "_").slice(0, 63);
  if (name && !/^[a-zA-Z_]/.test(name)) name = ("_" + name).slice(0, 63);
  return name || "files";
}
/** Where a connection points: host:port, or the folder for files. */
export const endpoint = (
  p: Pick<ConnectionProfile, "engine" | "host" | "port" | "local_dir">,
) => (engineOf(p.engine) === "files" ? p.local_dir : `${p.host}:${p.port}`);
export const engineOf = (e?: string): Engine =>
  e && e in engines ? (e as Engine) : "postgres";
/** "MariaDB 11.4.2", "PostgreSQL 18.1"… from the reported server version. */
export function dbVersion(engine: string | undefined, v?: string) {
  if (!v) return "—";
  const first = v.split(" ")[0];
  if (engineOf(engine) === "mysql")
    return /mariadb/i.test(v)
      ? "MariaDB " + first.split("-")[0]
      : "MySQL " + first;
  return engines[engineOf(engine)].label + " " + first;
}
export type Audit = {
  action: string;
  detail: string;
  created_at: string;
  actor?: string;
};
/** Access to one database ("*" = all) on one connection ("*" = all, "" = env). */
export type Grant = {
  connection_id: string;
  database: string;
  level: "view" | "operate";
};
export type User = {
  id: string;
  username: string;
  display_name: string;
  role: "admin" | "user";
  disabled: boolean;
  created_at: string;
  permissions: Grant[];
};
export type DB = { name: string; size: number };
export type Me = {
  role: "admin" | "user";
  user_id: string;
  username: string;
  display_name: string;
  /** Account defined in .env (admin or legacy viewer). */
  builtin: boolean;
  permissions: Grant[];
  timezone: string;
  /** TZ from .env; used when no timezone was picked in the UI. */
  default_timezone: string;
  restore_enabled: boolean;
  restore_host: string;
  max_upload_bytes: number;
  offsite_available: boolean;
};
/** Mirrors Principal.can on the server; db "*" needs an all-databases grant. */
export function can(
  me: Me | null,
  conn: string,
  db: string,
  level: "view" | "operate",
) {
  if (!me) return false;
  if (me.role === "admin") return true;
  return me.permissions.some(
    (g) =>
      (g.connection_id === "*" || g.connection_id === conn) &&
      (g.database === "*" || g.database === db) &&
      (level === "view" || g.level === "operate"),
  );
}
/** Any grant at this level on the connection. */
export function canConnection(
  me: Me | null,
  conn: string,
  level: "view" | "operate",
) {
  if (!me) return false;
  if (me.role === "admin") return true;
  return me.permissions.some(
    (g) =>
      (g.connection_id === "*" || g.connection_id === conn) &&
      (level === "view" || g.level === "operate"),
  );
}
export const initials = (name: string) =>
  name
    .trim()
    .split(/[\s._-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0].toUpperCase())
    .join("") || "?";

/** Live state of a connection, fetched from /api/databases. */
export type Probe = {
  state: "loading" | "ok" | "error";
  version?: string;
  dbs: DB[];
  error?: string;
};

export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const res = await fetch("/api" + path, {
    method,
    headers: {
      "Content-Type": "application/json",
      "X-Requested-With": "DBBackupManager",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = await res.json();
  if (!res.ok)
    throw new Error(translateServer(data.error) || t("error.request"));
  return data;
}

export const size = (n: number) =>
  n >= 1073741824
    ? (n / 1073741824).toFixed(2) + " GB"
    : n >= 1048576
      ? (n / 1048576).toFixed(1) + " MB"
      : n >= 1024
        ? (n / 1024).toFixed(1) + " KB"
        : n + " B";

export const date = (s: string) =>
  new Date(s).toLocaleString(locale(), {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

export function ago(s: string) {
  const diff = (Date.now() - new Date(s).getTime()) / 1000;
  if (diff < 60) return t("time.justNow");
  if (diff < 3600) return t("time.minutesAgo", { n: Math.floor(diff / 60) });
  if (diff < 86400) return t("time.hoursAgo", { n: Math.floor(diff / 3600) });
  if (diff < 86400 * 30)
    return t("time.daysAgo", { n: Math.floor(diff / 86400) });
  return date(s);
}

const statuses = ["completed", "running", "queued", "failed"];
export const statusLabel = (status: string) =>
  statuses.includes(status) ? t(("status." + status) as Key) : status;

const jobKinds = [
  "verify",
  "offsite",
  "offsite_pull",
  "restore",
  "retention",
  "scheduled",
];
export const jobLabel = (kind: string) =>
  t(("job." + (jobKinds.includes(kind) ? kind : "backup")) as Key);

const sources = ["scheduled", "upload", "cloud"];
export const sourceLabel = (source: string) =>
  t(("source." + (sources.includes(source) ? source : "manual")) as Key);

/** rclone backend type → readable provider name. */
export const providerName = (type: string) =>
  (
    ({
      drive: "Google Drive",
      onedrive: "OneDrive",
      dropbox: "Dropbox",
      s3: "S3",
      b2: "Backblaze B2",
      sftp: "SFTP",
      webdav: "WebDAV",
      box: "Box",
      pcloud: "pCloud",
      mega: "MEGA",
      azureblob: "Azure Blob",
      gcs: "Google Cloud Storage",
    }) as Record<string, string>
  )[type] || type;

// ---- Hash routing -------------------------------------------------------
// The env-based connection has an empty id, so it gets a placeholder key.
export const ENV_KEY = "_env";
export const connKey = (id: string) => encodeURIComponent(id || ENV_KEY);

export type ZoneTab =
  | "overview"
  | "databases"
  | "backups"
  | "schedules"
  | "activity"
  | "settings";
export type Route =
  | { view: "home" }
  | { view: "backups" }
  | { view: "schedules" }
  | { view: "activity" }
  | { view: "cloud" }
  | { view: "users" }
  | { view: "notifications" }
  | { view: "help"; section?: string }
  | { view: "zone"; id: string; tab: ZoneTab };

const zoneTabs: ZoneTab[] = [
  "overview",
  "databases",
  "backups",
  "schedules",
  "activity",
  "settings",
];

export function parseRoute(hash: string): Route {
  const parts = hash.replace(/^#\/?/, "").split("/").filter(Boolean);
  if (parts[0] === "c" && parts[1] !== undefined) {
    const raw = decodeURIComponent(parts[1]);
    const tab = zoneTabs.includes(parts[2] as ZoneTab)
      ? (parts[2] as ZoneTab)
      : "overview";
    return { view: "zone", id: raw === ENV_KEY ? "" : raw, tab };
  }
  if (parts[0] === "backups") return { view: "backups" };
  if (parts[0] === "schedules") return { view: "schedules" };
  if (parts[0] === "activity") return { view: "activity" };
  if (parts[0] === "cloud") return { view: "cloud" };
  if (parts[0] === "users") return { view: "users" };
  if (parts[0] === "notifications") return { view: "notifications" };
  if (parts[0] === "help") return { view: "help", section: parts[1] };
  return { view: "home" };
}

export function useRoute(): [Route, (path: string) => void] {
  const [route, setRoute] = useState(() => parseRoute(location.hash));
  useEffect(() => {
    const h = () => setRoute(parseRoute(location.hash));
    window.addEventListener("hashchange", h);
    return () => window.removeEventListener("hashchange", h);
  }, []);
  const go = (path: string) => {
    location.hash = path;
    window.scrollTo(0, 0);
  };
  return [route, go];
}
