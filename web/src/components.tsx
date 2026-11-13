import { useCallback, useEffect, useRef, useState, ReactNode } from "react";
import { GoogleDriveOAuth } from "./google-oauth";
import {
  ArrowDownToLine,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Globe,
  LogOut,
  Plus,
  UserRound,
  Users,
  Cloud,
  CloudUpload,
  Copy,
  KeyRound,
  Languages,
  CheckCircle2,
  Database,
  FlaskConical,
  LoaderCircle,
  ImageUp,
  MoreHorizontal,
  Palette,
  Pencil,
  Pause,
  Play,
  RefreshCw,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import {
  api,
  Audit,
  Backup,
  Branding,
  logoURL,
  ConnectionProfile,
  date,
  Engine,
  engineOf,
  filesItemName,
  engines,
  Destination,
  Grant,
  initials,
  Job,
  jobLabel,
  Me,
  Notifier,
  NotifierEvent,
  NotifierType,
  Probe,
  providerName,
  Schedule,
  size,
  sourceLabel,
  statusLabel,
  User,
} from "./lib";
import {
  getLang,
  Key,
  Lang,
  languages,
  setLang,
  t,
  translateServer,
  tx,
  useLang,
} from "./i18n";

export function Modal({
  title,
  children,
  close,
}: {
  title: string;
  children: ReactNode;
  close: () => void;
}) {
  return (
    <div className="overlay">
      <section
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modal-head">
          <h2>{title}</h2>
          <button
            className="icon-btn"
            aria-label={t("common.close")}
            onClick={close}
          >
            <X size={18} />
          </button>
        </div>
        <div className="modal-body">{children}</div>
      </section>
    </div>
  );
}

export function Empty({
  icon,
  title,
  text,
  action,
}: {
  icon: ReactNode;
  title: string;
  text: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty">
      <div className="empty-icon">{icon}</div>
      <h3>{title}</h3>
      <p>{text}</p>
      {action}
    </div>
  );
}

export function Stat({
  label,
  value,
  sub,
}: {
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <strong className="stat-value">{value}</strong>
      {sub && <span className="stat-sub">{sub}</span>}
    </div>
  );
}

/** Colored letter tile, like a site favicon placeholder. */
export function ConnAvatar({ name, small }: { name: string; small?: boolean }) {
  const palette = ["blue", "orange", "green", "purple", "teal", "pink"];
  const hash = [...name].reduce((a, c) => a + c.charCodeAt(0), 0);
  return (
    <span
      className={
        "conn-avatar " + palette[hash % palette.length] + (small ? " sm" : "")
      }
      aria-hidden
    >
      {(name.trim()[0] || "?").toUpperCase()}
    </span>
  );
}

export function ProbeBadge({ probe }: { probe?: Probe }) {
  if (!probe || probe.state === "loading")
    return (
      <span className="pill neutral">
        <LoaderCircle size={12} className="spin" /> {t("probe.checking")}
      </span>
    );
  if (probe.state === "ok")
    return (
      <span className="pill success">
        <i /> {t("common.active")}
      </span>
    );
  return (
    <span className="pill danger" title={probe.error}>
      <i /> {t("probe.down")}
    </span>
  );
}

export function JobBadge({ status }: { status: string }) {
  const tone =
    status === "completed"
      ? "success"
      : status === "failed"
        ? "danger"
        : status === "running"
          ? "info"
          : "warning";
  return (
    <span className={"pill " + tone}>
      {status === "running" ? (
        <LoaderCircle size={12} className="spin" />
      ) : (
        <i />
      )}
      {statusLabel(status)}
    </span>
  );
}

/** "⋯" dropdown menu. Items are [label, onClick, danger?]. */
export function Menu({
  items,
  label = t("menu.actions"),
}: {
  items: [string, () => void, boolean?][];
  label?: string;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const h = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", h);
    return () => document.removeEventListener("mousedown", h);
  }, [open]);
  return (
    <div className="menu" ref={ref}>
      <button
        className="icon-btn"
        aria-label={label}
        aria-expanded={open}
        onClick={(e) => {
          e.stopPropagation();
          setOpen(!open);
        }}
      >
        <MoreHorizontal size={18} />
      </button>
      {open && (
        <div className="menu-list" role="menu">
          {items.map(([text, fn, danger]) => (
            <button
              key={text}
              role="menuitem"
              className={danger ? "danger" : ""}
              onClick={(e) => {
                e.stopPropagation();
                setOpen(false);
                fn();
              }}
            >
              {text}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/** Rows per page in long tables. */
const PAGE_SIZE = 20;

/**
 * Splits a list into pages. The page is clamped when the list shrinks
 * (e.g. after a delete), so polling refreshes keep the user where they are.
 */
function usePaged<T>(items: T[]) {
  const [page, setPage] = useState(0);
  const pages = Math.max(1, Math.ceil(items.length / PAGE_SIZE));
  const current = Math.min(page, pages - 1);
  const start = current * PAGE_SIZE;
  const rows = items.slice(start, start + PAGE_SIZE);
  const pager =
    pages > 1 ? (
      <Pager
        page={current}
        pages={pages}
        from={start + 1}
        to={start + rows.length}
        total={items.length}
        onPage={setPage}
      />
    ) : null;
  return { rows, pager };
}

export function Pager({
  page,
  pages,
  from,
  to,
  total,
  onPage,
}: {
  page: number;
  pages: number;
  from: number;
  to: number;
  total: number;
  onPage: (p: number) => void;
}) {
  return (
    <div className="pager">
      <span className="muted">{t("pager.range", { from, to, total })}</span>
      <button
        className="icon-btn"
        disabled={page === 0}
        onClick={() => onPage(page - 1)}
        aria-label={t("pager.prev")}
        title={t("pager.prev")}
      >
        <ChevronLeft size={16} />
      </button>
      <span className="pager-page">
        {t("pager.page", { n: page + 1, total: pages })}
      </span>
      <button
        className="icon-btn"
        disabled={page >= pages - 1}
        onClick={() => onPage(page + 1)}
        aria-label={t("pager.next")}
        title={t("pager.next")}
      >
        <ChevronRight size={16} />
      </button>
    </div>
  );
}

export function BackupTable({
  backups,
  canOperate,
  restoreEnabled,
  connectionName,
  restore,
  remove,
  cloudCopies,
  toCloud,
  verify,
}: {
  backups: Backup[];
  /** Whether the signed-in user may download, restore or delete this backup. */
  canOperate: (b: Backup) => boolean;
  restoreEnabled: boolean;
  connectionName?: (id: string) => string;
  restore: (b: Backup) => void;
  remove: (b: Backup) => void;
  /** Names of the cloud destinations holding a copy of the backup. */
  cloudCopies: (b: Backup) => string[];
  /** Present when at least one cloud destination exists. */
  toCloud?: (b: Backup) => void;
  /** Opens the restore test dialog. */
  verify?: (b: Backup) => void;
}) {
  const { rows, pager } = usePaged(backups);
  return (
    <>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>{t("common.database")}</th>
              {connectionName && <th>{t("common.connection")}</th>}
              <th>{t("backups.source")}</th>
              <th>{t("common.time")}</th>
              <th className="num">{t("common.size")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {rows.map((b) => (
              <tr key={b.id}>
                <td>
                  <div className="cell-main">
                    <Database size={15} />
                    <strong>{b.database}</strong>
                    <span className="verified" title={"SHA-256 " + b.checksum}>
                      <Check size={10} /> SHA-256
                    </span>
                    {cloudCopies(b).length > 0 && (
                      <span
                        className="verified cloud"
                        title={t("backups.cloudCopies", {
                          list: cloudCopies(b).join(", "),
                        })}
                      >
                        <Cloud size={10} /> {cloudCopies(b).length}
                      </span>
                    )}
                    <VerifyBadge backup={b} />
                  </div>
                </td>
                {connectionName && <td>{connectionName(b.connection_id)}</td>}
                <td>{sourceLabel(b.source)}</td>
                <td>{date(b.created_at)}</td>
                <td className="num">{size(b.size)}</td>
                <td>
                  <div className="row-actions">
                    {canOperate(b) && (
                      <>
                        <a
                          className="icon-btn"
                          href={"/api/backups/" + b.id + "/download"}
                          title={t("action.download")}
                        >
                          <ArrowDownToLine size={16} />
                        </a>
                        {toCloud && (
                          <button
                            className="icon-btn"
                            title={t("backups.toCloud")}
                            onClick={() => toCloud(b)}
                          >
                            <CloudUpload size={16} />
                          </button>
                        )}
                        <button
                          className="icon-btn"
                          disabled={!restoreEnabled}
                          title={
                            restoreEnabled
                              ? t("backups.restore")
                              : t("backups.restoreDisabled")
                          }
                          onClick={() => restore(b)}
                        >
                          <RefreshCw size={15} />
                        </button>
                        {verify && (
                          <button
                            className="icon-btn"
                            disabled={!restoreEnabled}
                            title={
                              restoreEnabled
                                ? t("verify.action")
                                : t("backups.restoreDisabled")
                            }
                            onClick={() => verify(b)}
                          >
                            <FlaskConical size={15} />
                          </button>
                        )}
                        <button
                          className="icon-btn danger"
                          title={t("backups.delete")}
                          onClick={() => remove(b)}
                        >
                          <Trash2 size={15} />
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pager}
    </>
  );
}

/** Result of the latest restore test of a backup. */
export function VerifyBadge({ backup }: { backup: Backup }) {
  if (!backup.verify_status) return null;
  const ok = backup.verify_status === "passed";
  const when = backup.verified_at ? date(backup.verified_at) : "";
  return (
    <span
      className={"verified " + (ok ? "" : "fail")}
      title={
        (ok ? t("verify.passed") : t("verify.failed")) +
        (when ? " · " + when : "") +
        "\n" +
        translateServer(backup.verify_detail || "")
      }
    >
      {ok ? <FlaskConical size={10} /> : <X size={10} />}
      {ok ? t("verify.badgeOk") : t("verify.badgeFail")}
    </span>
  );
}

export function ScheduleTable({
  schedules,
  canOperate,
  connectionName,
  destinationName,
  toggle,
  edit,
  remove,
}: {
  schedules: Schedule[];
  canOperate: (s: Schedule) => boolean;
  connectionName?: (id: string) => string;
  destinationName: (id: string) => string;
  toggle: (s: Schedule) => void;
  edit: (s: Schedule) => void;
  remove: (s: Schedule) => void;
}) {
  return (
    <div className="table-scroll">
      <table>
        <thead>
          <tr>
            <th>{t("common.database")}</th>
            {connectionName && <th>{t("common.connection")}</th>}
            <th>Cron</th>
            <th>{t("schedules.nextRun")}</th>
            <th>{t("schedules.retention")}</th>
            <th>Cloud</th>
            <th>{t("common.status")}</th>
            <th />
          </tr>
        </thead>
        <tbody>
          {schedules.map((s) => (
            <tr key={s.id}>
              <td>
                <div className="cell-main">
                  <Database size={15} />
                  <strong>
                    {s.database === "*" ? t("common.allDatabases") : s.database}
                  </strong>
                  {s.verify && (
                    <span
                      className="verified neutral"
                      title={t("schedule.verifyOn", {
                        server: connectionName
                          ? connectionName(
                              s.verify_connection_id || s.connection_id,
                            )
                          : "",
                      })}
                    >
                      <FlaskConical size={10} /> {t("verify.scheduleBadge")}
                    </span>
                  )}
                </div>
              </td>
              {connectionName && <td>{connectionName(s.connection_id)}</td>}
              <td>
                <code>{s.cron}</code>
              </td>
              <td>{s.next_run ? date(s.next_run) : "—"}</td>
              <td>{t("schedules.days", { n: s.retention_days })}</td>
              <td>
                {s.destination_id ? (
                  <span className="cloud-name">
                    <Cloud size={14} /> {destinationName(s.destination_id)}
                  </span>
                ) : (
                  <span className="muted">—</span>
                )}
              </td>
              <td>
                <span className={"pill " + (s.enabled ? "success" : "neutral")}>
                  <i />
                  {s.enabled ? t("schedules.enabled") : t("schedules.paused")}
                </span>
              </td>
              <td>
                {canOperate(s) && (
                  <div className="row-actions">
                    <button
                      className="icon-btn"
                      title={t("schedules.edit")}
                      onClick={() => edit(s)}
                    >
                      <Pencil size={15} />
                    </button>
                    <button
                      className="icon-btn"
                      title={
                        s.enabled ? t("schedules.pause") : t("schedules.enable")
                      }
                      onClick={() => toggle(s)}
                    >
                      {s.enabled ? <Pause size={16} /> : <Play size={16} />}
                    </button>
                    <button
                      className="icon-btn danger"
                      title={t("schedules.delete")}
                      onClick={() => remove(s)}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function JobTable({
  jobs,
  connectionName,
}: {
  jobs: Job[];
  connectionName?: (id: string) => string;
}) {
  const { rows, pager } = usePaged(jobs);
  return (
    <>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>{t("menu.actions")}</th>
              <th>{t("common.database")}</th>
              {connectionName && <th>{t("common.connection")}</th>}
              <th>{t("jobs.started")}</th>
              <th>{t("jobs.finished")}</th>
              <th>{t("common.status")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((j) => (
              <tr key={j.id}>
                <td>
                  <strong>{jobLabel(j.kind)}</strong>
                  {!!j.attempt && (
                    <span className="attempt">
                      {t("jobs.attempt", { n: j.attempt + 1 })}
                    </span>
                  )}
                  {j.error && (
                    <details className="job-error">
                      <summary>{t("jobs.errorDetail")}</summary>
                      <pre>{translateServer(j.error)}</pre>
                    </details>
                  )}
                </td>
                <td>{j.database || "—"}</td>
                {connectionName && <td>{connectionName(j.connection_id)}</td>}
                <td>{date(j.created_at)}</td>
                <td>{j.finished_at ? date(j.finished_at) : "—"}</td>
                <td>
                  <JobBadge status={j.status} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pager}
    </>
  );
}

export function AuditTable({ audit }: { audit: Audit[] }) {
  const { rows, pager } = usePaged(audit);
  return (
    <>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th>{t("audit.event")}</th>
              <th>{t("audit.actor")}</th>
              <th>{t("audit.detail")}</th>
              <th>{t("common.time")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((a, i) => (
              <tr key={a.created_at + i}>
                <td>
                  <code>{a.action}</code>
                </td>
                <td>
                  {a.actor || (
                    <span className="muted">{t("audit.system")}</span>
                  )}
                </td>
                <td className="wrap">{a.detail}</td>
                <td>{date(a.created_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {pager}
    </>
  );
}

export function ConnectionForm({
  profile,
  saved,
  setBusy,
}: {
  profile: ConnectionProfile | null;
  saved: (profile: ConnectionProfile) => Promise<void>;
  setBusy: (busy: boolean) => void;
}) {
  const initial = engines[engineOf(profile?.engine)];
  const [form, setForm] = useState({
    id: profile?.id || "",
    engine: engineOf(profile?.engine),
    server_dir: profile?.server_dir || "",
    local_dir: profile?.local_dir || "",
    name: profile?.name || "",
    host: profile?.host || "",
    port: profile?.port || initial.port,
    username: profile?.username || "",
    password: "",
    database: profile?.database || initial.database,
    sslmode: profile?.sslmode || initial.sslmode,
    has_password: profile?.has_password || false,
    restore_username: profile?.restore_username || "",
    restore_password: "",
    has_restore_password: profile?.has_restore_password || false,
  });
  const [working, setWorking] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  function field(key: keyof typeof form, value: string | boolean) {
    setForm((f) => ({ ...f, [key]: value }));
    setMessage("");
    setError("");
  }
  /** Switches engine, replacing values still at the old engine's defaults. */
  function chooseEngine(next: Engine) {
    setForm((f) => {
      const from = engines[f.engine];
      const to = engines[next];
      return {
        ...f,
        engine: next,
        port: !f.port || f.port === from.port ? to.port : f.port,
        database:
          !to.database || !f.database || f.database === from.database
            ? to.database
            : f.database,
        sslmode: to.sslModes.includes(f.sslmode) ? f.sslmode : to.sslmode,
      };
    });
    setMessage("");
    setError("");
  }
  /** Sets the files source folder. On a new connection the item name follows
   * the folder until the user types their own. Existing connections keep
   * their name: schedules, grants and backups refer to it. */
  function chooseSourceDir(dir: string) {
    setForm((f) => ({
      ...f,
      local_dir: dir,
      database:
        !profile && f.database === filesItemName(f.local_dir)
          ? filesItemName(dir)
          : f.database,
    }));
    setMessage("");
    setError("");
  }
  const meta = engines[form.engine];
  const userRequired = form.engine !== "mongodb";
  const isFiles = form.engine === "files";
  async function submit(test: boolean) {
    setWorking(true);
    setBusy(true);
    setMessage("");
    setError("");
    try {
      if (test) {
        await api("/connections/test", "POST", form);
        setMessage(
          isFiles
            ? t("connForm.testOkFiles")
            : form.restore_username
              ? t("connForm.testOkRestore")
              : t("connForm.testOk", { engine: meta.label }),
        );
      } else {
        const c = await api<ConnectionProfile>(
          profile ? "/connections/" + profile.id : "/connections",
          profile ? "PUT" : "POST",
          form,
        );
        await saved(c);
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setWorking(false);
      setBusy(false);
    }
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void submit(false);
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      {message && (
        <div className="notice success" role="status">
          <CheckCircle2 size={16} />
          {message}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <h3 className="form-section">{t("settings.server")}</h3>
        <label>
          {t("connForm.engine")}
          <select
            value={form.engine}
            onChange={(e) => chooseEngine(e.target.value as Engine)}
          >
            {(Object.keys(engines) as Engine[]).map((k) => (
              <option key={k} value={k}>
                {engines[k].label}
              </option>
            ))}
          </select>
        </label>
        <label>
          {t("connForm.name")}
          <input
            autoFocus
            value={form.name}
            maxLength={100}
            placeholder={t("connForm.namePlaceholder")}
            onChange={(e) => field("name", e.target.value)}
            required
          />
        </label>
        {isFiles ? (
          <>
            <label>
              {t("connForm.sourceDir")}
              <input
                value={form.local_dir}
                onChange={(e) => chooseSourceDir(e.target.value)}
                placeholder="/sources/uploads"
                pattern="/.+"
                required
              />
              <small>{t("connForm.sourceDirHint")}</small>
            </label>
            <label>
              {t("connForm.itemName")}
              <input
                value={form.database}
                onChange={(e) => field("database", e.target.value)}
                pattern="[a-zA-Z_][a-zA-Z0-9_-]{0,62}"
                required
              />
              <small>{t("connForm.itemNameHint")}</small>
            </label>
            <h3 className="form-section">
              Restore <span className="optional">{t("common.optional")}</span>
            </h3>
            <label>
              {t("connForm.restoreDir")}
              <input
                value={form.server_dir}
                onChange={(e) => field("server_dir", e.target.value)}
                placeholder="/restore"
                pattern="/.+"
              />
              <small>{t("connForm.restoreDirHint")}</small>
            </label>
            <div className="form-note">
              <ShieldCheck size={16} />
              {t("connForm.filesNote")}
            </div>
          </>
        ) : (
          <>
            <div className="form-row">
              <label>
                {t("common.host")}
                <input
                  value={form.host}
                  placeholder={t("connForm.hostPlaceholder")}
                  onChange={(e) => field("host", e.target.value)}
                  required
                />
              </label>
              <label className="narrow">
                Port
                <input
                  type="number"
                  min={1}
                  max={65535}
                  value={form.port}
                  onChange={(e) => field("port", e.target.value)}
                  required
                />
              </label>
            </div>
            <div className="form-row">
              <label>
                {t("common.account")}
                <input
                  autoComplete="off"
                  value={form.username}
                  onChange={(e) => field("username", e.target.value)}
                  maxLength={128}
                  required={userRequired}
                />
                {!userRequired && <small>{t("connForm.mongoNoAuth")}</small>}
              </label>
              <label>
                {t("login.password")}
                <input
                  type="password"
                  autoComplete="new-password"
                  value={form.password}
                  onChange={(e) => field("password", e.target.value)}
                  placeholder={
                    form.has_password
                      ? t("common.keepBlank")
                      : t("login.password")
                  }
                />
              </label>
            </div>
            {profile?.has_password && (
              <label className="checkbox">
                <input
                  type="checkbox"
                  checked={!form.has_password}
                  onChange={(e) => field("has_password", !e.target.checked)}
                />
                {t("connForm.clearPassword")}
              </label>
            )}
            <div className="form-row">
              {meta.database && (
                <label>
                  {form.engine === "mongodb"
                    ? t("connForm.authDatabase")
                    : t("connForm.database")}
                  <input
                    value={form.database}
                    onChange={(e) => field("database", e.target.value)}
                    pattern="[a-zA-Z_][a-zA-Z0-9_-]{0,62}"
                    required
                  />
                  <small>
                    {form.engine === "mongodb"
                      ? t("connForm.databaseHintMongo")
                      : form.engine === "mssql"
                        ? t("connForm.databaseHintMssql")
                        : t("connForm.databaseHint")}
                  </small>
                </label>
              )}
              <label>
                SSL mode
                <select
                  value={form.sslmode}
                  onChange={(e) => field("sslmode", e.target.value)}
                >
                  {meta.sslModes.map((mode) => (
                    <option key={mode}>{mode}</option>
                  ))}
                </select>
              </label>
            </div>
            {form.engine === "mssql" && (
              <>
                <h3 className="form-section">{t("connForm.sharedDir")}</h3>
                <div className="form-row">
                  <label>
                    {t("connForm.serverDir")}
                    <input
                      value={form.server_dir}
                      onChange={(e) => field("server_dir", e.target.value)}
                      placeholder="/var/opt/mssql/backup"
                      required
                    />
                  </label>
                  <label>
                    {t("connForm.localDir")}
                    <input
                      value={form.local_dir}
                      onChange={(e) => field("local_dir", e.target.value)}
                      placeholder="/mssql-backup"
                      pattern="/.*"
                      required
                    />
                  </label>
                </div>
                <small className="hint">{t("connForm.sharedDirHint")}</small>
              </>
            )}
            <h3 className="form-section">
              Restore <span className="optional">{t("common.optional")}</span>
            </h3>
            <div className="form-row">
              <label>
                {t("connForm.restoreUser")}
                <input
                  value={form.restore_username}
                  maxLength={128}
                  autoComplete="off"
                  onChange={(e) => field("restore_username", e.target.value)}
                  placeholder="restore_user"
                />
              </label>
              <label>
                {t("connForm.restorePassword")}
                <input
                  type="password"
                  autoComplete="new-password"
                  value={form.restore_password}
                  onChange={(e) => field("restore_password", e.target.value)}
                  placeholder={
                    form.has_restore_password
                      ? t("common.keepBlank")
                      : t("connForm.restorePassword")
                  }
                />
              </label>
            </div>
            <small className="hint">
              {form.engine === "mysql"
                ? t("connForm.restoreHintMysql")
                : form.engine === "mongodb"
                  ? t("connForm.restoreHintMongo")
                  : form.engine === "mssql"
                    ? t("connForm.restoreHintMssql")
                    : t("connForm.restoreHint")}
            </small>
            {profile?.has_restore_password && (
              <label className="checkbox">
                <input
                  type="checkbox"
                  checked={!form.has_restore_password}
                  onChange={(e) =>
                    field("has_restore_password", !e.target.checked)
                  }
                />
                {t("connForm.clearRestorePassword")}
              </label>
            )}
            <div className="form-note">
              <ShieldCheck size={16} />
              {t("connForm.hostNote")}
            </div>
          </>
        )}
        <div className="modal-actions">
          <button
            type="button"
            className="secondary"
            onClick={() => void submit(true)}
            disabled={
              working ||
              !form.name ||
              (isFiles
                ? !form.local_dir
                : !form.host || (userRequired && !form.username))
            }
          >
            {working ? (
              <LoaderCircle size={16} className="spin" />
            ) : (
              <RefreshCw size={16} />
            )}
            {t("settings.check")}
          </button>
          <button className="primary" disabled={working}>
            <Check size={16} />
            {profile ? t("common.saveChanges") : t("conn.add")}
          </button>
        </div>
      </fieldset>
    </form>
  );
}

const rcloneLink = (c: string) => (
  <a href="https://rclone.org/install/" target="_blank" rel="noreferrer">
    {c}
  </a>
);
const backendsLink = (c: string) => (
  <a href="https://rclone.org/overview/" target="_blank" rel="noreferrer">
    {c}
  </a>
);

/** Setup steps per provider; built per render so they follow the language. */
const providerHelp = (): Record<
  string,
  { label: string; steps: ReactNode; placeholder: string }
> => ({
  drive: {
    label: "Google Drive",
    steps: (
      <ol>
        <li>{tx("help.drive1", { rclone: rcloneLink })}</li>
        <li>{tx("help.drive2")}</li>
        <li>{tx("help.drive3")}</li>
      </ol>
    ),
    placeholder:
      '[gdrive]\ntype = drive\nscope = drive.file\ntoken = {"access_token":"…","refresh_token":"…"}',
  },
  onedrive: {
    label: "OneDrive",
    steps: (
      <ol>
        <li>{tx("help.onedrive1")}</li>
        <li>{tx("help.onedrive2")}</li>
        <li>{tx("help.onedrive3")}</li>
      </ol>
    ),
    placeholder:
      '[onedrive]\ntype = onedrive\ntoken = {"access_token":"…"}\ndrive_id = …\ndrive_type = personal',
  },
  s3: {
    label: "S3 / R2 / MinIO",
    steps: (
      <ol>
        <li>{tx("help.s31")}</li>
        <li>{tx("help.s32")}</li>
      </ol>
    ),
    placeholder:
      "type = s3\nprovider = AWS\naccess_key_id = …\nsecret_access_key = …\nregion = ap-southeast-1",
  },
  other: {
    label: t("help.other"),
    steps: <p>{tx("help.otherText", { link: backendsLink })}</p>,
    placeholder: "type = dropbox\ntoken = {…}",
  },
});

export function DestinationForm({
  destination,
  saved,
  setBusy,
}: {
  destination: Destination | null;
  saved: (d: Destination) => Promise<void>;
  setBusy: (busy: boolean) => void;
}) {
  const helps = providerHelp();
  const [form, setForm] = useState({
    id: destination?.id || "",
    name: destination?.name || "",
    config: "",
    google_oauth_id: "",
    remote_path: destination?.remote_path ?? "db-backups",
    encryption: destination?.encryption || "recipient",
    recipients: destination?.recipients || "",
    passphrase: "",
    has_passphrase: destination?.has_passphrase || false,
    retention_days: destination?.retention_days ?? 30,
  });
  const [provider, setProvider] = useState(
    destination && destination.type in helps
      ? destination.type
      : destination
        ? "other"
        : "drive",
  );
  const [confirm, setConfirm] = useState("");
  const [driveAuth, setDriveAuth] = useState("oauth");
  const [oauthPending, setOAuthPending] = useState(false);
  const oauthReady = useCallback((id: string) => {
    setForm((f) => ({ ...f, config: "", google_oauth_id: id }));
    setError("");
    setMessage("");
  }, []);
  const [privateKey, setPrivateKey] = useState("");
  const [keySaved, setKeySaved] = useState(false);
  const [working, setWorking] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const help = helps[provider];
  function field(key: keyof typeof form, value: string | number | boolean) {
    setForm((f) => ({ ...f, [key]: value }));
    setMessage("");
    setError("");
  }
  const newPassphrase = form.encryption === "passphrase" && form.passphrase;
  async function generate() {
    setError("");
    try {
      const k = await api<{ public_key: string; private_key: string }>(
        "/destinations/keygen",
        "POST",
      );
      setPrivateKey(k.private_key);
      setKeySaved(false);
      field("recipients", k.public_key);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  function downloadKey() {
    const blob = new Blob(
      [
        `# ${t("keyfile.title", { name: form.name || "cloud" })}\n` +
          `# public key: ${form.recipients}\n` +
          `# ${t("keyfile.decrypt")}: age -d -i this-file.txt file.dump.age > file.dump\n` +
          privateKey +
          "\n",
      ],
      { type: "text/plain" },
    );
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "db-backup-age-key.txt";
    a.click();
    URL.revokeObjectURL(a.href);
  }
  async function submit(test: boolean) {
    if (oauthPending) return;
    const oauth = provider === "drive" && driveAuth === "oauth";
    if (
      oauth &&
      !form.google_oauth_id &&
      (!destination || destination.type !== "drive")
    ) {
      setError(t("googleOAuth.required"));
      return;
    }
    const payload = {
      ...form,
      config: oauth ? "" : form.config,
      google_oauth_id: oauth ? form.google_oauth_id : "",
    };
    if (newPassphrase && form.passphrase !== confirm) {
      setError(t("destForm.passMismatch"));
      return;
    }
    if (!test && privateKey && !keySaved) {
      setError(t("destForm.confirmKeySaved"));
      return;
    }
    setWorking(true);
    setBusy(true);
    setMessage("");
    setError("");
    try {
      if (test) {
        await api("/destinations/test", "POST", payload);
        setMessage(t("destForm.testOk"));
      } else {
        const d = await api<Destination>(
          destination ? "/destinations/" + destination.id : "/destinations",
          destination ? "PUT" : "POST",
          payload,
        );
        await saved(d);
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setWorking(false);
      setBusy(false);
    }
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void submit(false);
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      {message && (
        <div className="notice success" role="status">
          <CheckCircle2 size={16} />
          {message}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <h3 className="form-section">{t("destForm.storage")}</h3>
        <label>
          {t("destForm.name")}
          <input
            autoFocus
            value={form.name}
            maxLength={100}
            placeholder={t("destForm.namePlaceholder")}
            onChange={(e) => field("name", e.target.value)}
            required
          />
        </label>
        <div className="preset-row" role="tablist">
          {Object.entries(helps).map(([k, v]) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={provider === k}
              className={"chip" + (provider === k ? " on" : "")}
              onClick={() => setProvider(k)}
            >
              {v.label}
            </button>
          ))}
        </div>
        {provider === "drive" && (
          <div className="segmented" role="tablist">
            {[
              ["oauth", t("googleOAuth.option")],
              ["rclone", t("destForm.config")],
            ].map(([key, label]) => (
              <button
                key={key}
                type="button"
                role="tab"
                aria-selected={driveAuth === key}
                className={driveAuth === key ? "on" : ""}
                onClick={() => setDriveAuth(key)}
              >
                {label}
              </button>
            ))}
          </div>
        )}
        {provider === "drive" && driveAuth === "oauth" ? (
          <GoogleDriveOAuth
            ready={!!form.google_oauth_id}
            connected={destination?.type === "drive"}
            onReady={oauthReady}
            onPending={setOAuthPending}
          />
        ) : (
          <>
            <div className="form-note steps">{help.steps}</div>
            <label>
              {t("destForm.config")}
              <textarea
                className="mono"
                rows={6}
                spellCheck={false}
                autoComplete="off"
                value={form.config}
                onChange={(e) => field("config", e.target.value)}
                placeholder={
                  destination
                    ? t("destForm.configKeep", {
                        provider: providerName(destination.type),
                      })
                    : help.placeholder
                }
                required={!destination}
              />
              <small>{t("destForm.configHint")}</small>
            </label>
          </>
        )}
        <div className="form-row">
          <label>
            {t("destForm.remotePath")}
            <input
              value={form.remote_path}
              placeholder={
                provider === "s3" ? "bucket/db-backups" : "db-backups"
              }
              onChange={(e) => field("remote_path", e.target.value)}
            />
            <small>{t("destForm.remotePathHint")}</small>
          </label>
          <label className="narrow">
            {t("destForm.keepDays")}
            <input
              type="number"
              min={0}
              max={3650}
              value={form.retention_days}
              onChange={(e) => field("retention_days", Number(e.target.value))}
              required
            />
            <small>{t("destForm.forever")}</small>
          </label>
        </div>

        <h3 className="form-section">{t("cloud.encryption")}</h3>
        <div className="segmented" role="tablist">
          {(
            [
              ["recipient", t("destForm.publicKeyRec")],
              ["passphrase", t("destForm.passphrase")],
            ] as const
          ).map(([k, text]) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={form.encryption === k}
              className={form.encryption === k ? "on" : ""}
              onClick={() => field("encryption", k)}
            >
              {text}
            </button>
          ))}
        </div>
        {form.encryption === "recipient" ? (
          <>
            <div className="form-note">
              <KeyRound size={16} />
              <span>{tx("destForm.recipientNote")}</span>
            </div>
            <label>
              {t("destForm.publicKey")}
              <textarea
                className="mono"
                rows={2}
                spellCheck={false}
                value={form.recipients}
                onChange={(e) => {
                  field("recipients", e.target.value);
                  setPrivateKey("");
                }}
                placeholder={t("destForm.publicKeyPlaceholder")}
                required
              />
            </label>
            {!privateKey ? (
              <button
                type="button"
                className="secondary"
                onClick={() => void generate()}
              >
                <KeyRound size={16} /> {t("destForm.genKey")}
              </button>
            ) : (
              <div className="key-box">
                <strong>{t("destForm.privateOnce")}</strong>
                <div className="checksum">{privateKey}</div>
                <div className="preset-row">
                  <button
                    type="button"
                    className="secondary sm"
                    onClick={() =>
                      void navigator.clipboard?.writeText(privateKey)
                    }
                  >
                    <Copy size={14} /> {t("destForm.copy")}
                  </button>
                  <button
                    type="button"
                    className="secondary sm"
                    onClick={downloadKey}
                  >
                    <ArrowDownToLine size={14} /> {t("destForm.downloadTxt")}
                  </button>
                </div>
                <small className="hint">{t("destForm.privateHint")}</small>
                <label className="checkbox">
                  <input
                    type="checkbox"
                    checked={keySaved}
                    onChange={(e) => setKeySaved(e.target.checked)}
                  />
                  {t("destForm.keySaved")}
                </label>
              </div>
            )}
          </>
        ) : (
          <>
            <div className="form-note">
              <KeyRound size={16} />
              <span>{t("destForm.passphraseNote")}</span>
            </div>
            <div className="form-row">
              <label>
                {t("destForm.passphrase")}
                <input
                  type="password"
                  autoComplete="new-password"
                  value={form.passphrase}
                  minLength={16}
                  onChange={(e) => field("passphrase", e.target.value)}
                  placeholder={
                    form.has_passphrase
                      ? t("common.keepBlank")
                      : t("destForm.minChars")
                  }
                  required={!form.has_passphrase}
                />
              </label>
              <label>
                {t("destForm.repeat")}
                <input
                  type="password"
                  autoComplete="new-password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  required={!!form.passphrase}
                />
              </label>
            </div>
          </>
        )}
        {destination && (
          <small className="hint">{t("destForm.rekeyHint")}</small>
        )}
        <div className="modal-actions">
          <button
            type="button"
            className="secondary"
            onClick={() => void submit(true)}
            disabled={
              working ||
              oauthPending ||
              (!destination && !form.config && !form.google_oauth_id)
            }
          >
            {working ? (
              <LoaderCircle size={16} className="spin" />
            ) : (
              <RefreshCw size={16} />
            )}
            {t("destForm.test")}
          </button>
          <button className="primary" disabled={working || oauthPending}>
            <Check size={16} />
            {destination ? t("common.saveChanges") : t("dest.add")}
          </button>
        </div>
      </fieldset>
    </form>
  );
}

// ------------------------------------------------------------------ users --

export const roleLabel = (me: Pick<Me, "role" | "builtin" | "username">) =>
  me.role === "admin"
    ? t("role.admin")
    : me.builtin && me.username === "viewer"
      ? t("role.viewer")
      : t("role.user");

const levelLabel = (l: Grant["level"]) =>
  l === "operate" ? t("level.operate") : t("level.view");

/** Chips listing a user's grants. */
export function GrantList({
  grants,
  connectionName,
}: {
  grants: Grant[];
  connectionName: (id: string) => string;
}) {
  if (!grants.length) return <span className="muted">{t("grants.none")}</span>;
  return (
    <div className="grant-list">
      {grants.map((g) => (
        <span
          key={g.connection_id + "/" + g.database}
          className={"grant " + g.level}
        >
          {g.connection_id === "*"
            ? t("common.allConnections")
            : connectionName(g.connection_id)}
          <span className="sep">/</span>
          {g.database === "*" ? t("grants.allDbs") : g.database}
          <b>{levelLabel(g.level)}</b>
        </span>
      ))}
    </div>
  );
}

/** Avatar button in the top bar; holds account info and account actions. */
export function UserMenu({
  me,
  openProfile,
  openUsers,
  openTimezone,
  openBranding,
  logout,
}: {
  me: Me;
  openProfile: () => void;
  openUsers?: () => void;
  /** Admins only: product name and logo. */
  openBranding?: () => void;
  /** Admins only: the scheduler timezone is shared by all users. */
  openTimezone?: () => void;
  logout: () => void;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const click = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    const key = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("mousedown", click);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("mousedown", click);
      document.removeEventListener("keydown", key);
    };
  }, [open]);
  const pick = (fn: () => void) => () => {
    setOpen(false);
    fn();
  };
  const name = me.display_name || me.username;
  return (
    <div className="user-menu" ref={ref}>
      <button
        className="user"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        <span className="avatar">{initials(name)}</span>
        <span className="user-text">{name}</span>
        <ChevronDown size={15} className="user-chev" />
      </button>
      {open && (
        <div className="user-pop" role="menu">
          <div className="user-pop-head">
            <span className="avatar lg">{initials(name)}</span>
            <div>
              <strong>{name}</strong>
              <small>@{me.username}</small>
            </div>
            <span
              className={"pill " + (me.role === "admin" ? "info" : "neutral")}
            >
              {roleLabel(me)}
            </span>
          </div>
          <div className="user-pop-info">
            <Globe size={15} />
            <span>{t("menu.timezone")}</span>
            <b>{me.timezone}</b>
          </div>
          <div className="user-pop-info">
            <Languages size={15} />
            <span>{t("common.language")}</span>
            <LanguageSelect />
          </div>
          <div className="user-pop-sep" />
          <button role="menuitem" onClick={pick(openProfile)}>
            <UserRound size={16} /> {t("menu.profile")}
          </button>
          {openTimezone && (
            <button role="menuitem" onClick={pick(openTimezone)}>
              <Globe size={16} /> {t("menu.changeTimezone")}
            </button>
          )}
          {openBranding && (
            <button role="menuitem" onClick={pick(openBranding)}>
              <Palette size={16} /> {t("menu.branding")}
            </button>
          )}
          {openUsers && (
            <button role="menuitem" onClick={pick(openUsers)}>
              <Users size={16} /> {t("menu.users")}
            </button>
          )}
          <div className="user-pop-sep" />
          <button role="menuitem" className="danger" onClick={pick(logout)}>
            <LogOut size={16} /> {t("menu.logout")}
          </button>
        </div>
      )}
    </div>
  );
}

export function PasswordForm({
  saved,
  setBusy,
}: {
  saved: () => void;
  setBusy: (busy: boolean) => void;
}) {
  const [form, setForm] = useState({ current: "", next: "", confirm: "" });
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const mismatch = form.confirm !== "" && form.next !== form.confirm;
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        setWorking(true);
        setBusy(true);
        setError("");
        try {
          await api("/me/password", "POST", {
            current_password: form.current,
            new_password: form.next,
          });
          setForm({ current: "", next: "", confirm: "" });
          saved();
        } catch (e) {
          setError((e as Error).message);
        } finally {
          setWorking(false);
          setBusy(false);
        }
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <label>
          {t("pw.current")}
          <input
            type="password"
            autoComplete="current-password"
            value={form.current}
            onChange={(e) => setForm({ ...form, current: e.target.value })}
            required
          />
        </label>
        <div className="form-row">
          <label>
            {t("pw.new")}
            <input
              type="password"
              autoComplete="new-password"
              minLength={12}
              maxLength={72}
              value={form.next}
              onChange={(e) => setForm({ ...form, next: e.target.value })}
              required
            />
            <small>{t("pw.hint")}</small>
          </label>
          <label>
            {t("pw.confirm")}
            <input
              type="password"
              autoComplete="new-password"
              value={form.confirm}
              onChange={(e) => setForm({ ...form, confirm: e.target.value })}
              required
            />
            {mismatch && (
              <small className="error-text">{t("pw.mismatch")}</small>
            )}
          </label>
        </div>
      </fieldset>
      <div className="modal-actions">
        <button
          className="primary"
          disabled={working || mismatch || !form.next}
        >
          {working ? (
            <LoaderCircle size={16} className="spin" />
          ) : (
            <KeyRound size={16} />
          )}
          {t("profile.changePassword")}
        </button>
      </div>
    </form>
  );
}

export function UserForm({
  user,
  selfId,
  profiles,
  probes,
  saved,
  cancel,
  setBusy,
}: {
  user: User | null;
  selfId: string;
  profiles: ConnectionProfile[];
  probes: Record<string, Probe>;
  saved: (u: User) => Promise<void>;
  cancel: () => void;
  setBusy: (busy: boolean) => void;
}) {
  const [form, setForm] = useState({
    username: user?.username || "",
    display_name: user?.display_name || "",
    password: "",
    role: user?.role || ("user" as User["role"]),
    disabled: user?.disabled || false,
  });
  const [grants, setGrants] = useState<Grant[]>(
    user?.permissions.length
      ? user.permissions
      : [
          {
            connection_id: profiles[0]?.id ?? "*",
            database: "*",
            level: "view",
          },
        ],
  );
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  const self = !!user && user.id === selfId;
  const setGrant = (i: number, patch: Partial<Grant>) =>
    setGrants((gs) => gs.map((g, j) => (j === i ? { ...g, ...patch } : g)));
  return (
    <form
      onSubmit={async (e) => {
        e.preventDefault();
        setWorking(true);
        setBusy(true);
        setError("");
        try {
          const u = await api<User>(
            user ? "/users/" + user.id : "/users",
            user ? "PUT" : "POST",
            {
              ...form,
              permissions: grants.map((g) => ({
                ...g,
                database: g.database.trim() || "*",
              })),
            },
          );
          await saved(u);
        } catch (e) {
          setError((e as Error).message);
        } finally {
          setWorking(false);
          setBusy(false);
        }
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <h3 className="form-section">{t("common.account")}</h3>
        <div className="form-row">
          <label>
            {t("login.username")}
            <input
              autoFocus={!user}
              autoComplete="off"
              value={form.username}
              pattern="[a-zA-Z0-9][a-zA-Z0-9._\-]{1,31}"
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              required
            />
          </label>
          <label>
            {t("userForm.displayName")}
            <input
              value={form.display_name}
              maxLength={100}
              placeholder={form.username || t("userForm.displayPlaceholder")}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
            />
          </label>
        </div>
        <label>
          {user ? t("userForm.resetPassword") : t("login.password")}
          <input
            type="password"
            autoComplete="new-password"
            minLength={12}
            maxLength={72}
            value={form.password}
            placeholder={user ? t("common.keepBlank") : t("userForm.pwLength")}
            onChange={(e) => setForm({ ...form, password: e.target.value })}
            required={!user}
          />
          {user && !self && <small>{t("userForm.resetHint")}</small>}
        </label>
        <div
          className="role-pick"
          role="radiogroup"
          aria-label={t("users.role")}
        >
          {(
            [
              ["user", t("role.user"), t("userForm.userText")],
              ["admin", t("role.admin"), t("userForm.adminText")],
            ] as const
          ).map(([value, title, text]) => (
            <label
              key={value}
              className={"role-option" + (form.role === value ? " on" : "")}
            >
              <input
                type="radio"
                name="role"
                checked={form.role === value}
                disabled={self}
                onChange={() => setForm({ ...form, role: value })}
              />
              <span>
                <strong>{title}</strong>
                <small>{text}</small>
              </span>
            </label>
          ))}
        </div>
        {user && (
          <label className="checkbox">
            <input
              type="checkbox"
              checked={form.disabled}
              disabled={self}
              onChange={(e) => setForm({ ...form, disabled: e.target.checked })}
            />
            {t("userForm.lock")}
          </label>
        )}
        {form.role === "user" && (
          <>
            <h3 className="form-section">{t("userForm.grants")}</h3>
            <div className="grant-editor">
              {grants.map((g, i) => {
                const dbs =
                  g.connection_id !== "*" &&
                  probes[g.connection_id]?.state === "ok"
                    ? probes[g.connection_id].dbs
                    : [];
                return (
                  <div className="grant-row" key={i}>
                    <select
                      aria-label={t("common.connection")}
                      value={g.connection_id}
                      onChange={(e) =>
                        setGrant(i, { connection_id: e.target.value })
                      }
                    >
                      <option value="*">{t("common.allConnections")}</option>
                      {profiles.map((p) => (
                        <option key={p.id || "_env"} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                    </select>
                    <input
                      aria-label={t("common.database")}
                      list={"grant-dbs-" + i}
                      value={g.database === "*" ? "" : g.database}
                      placeholder={t("common.allDatabases")}
                      pattern="[a-zA-Z_][a-zA-Z0-9_\-]{0,62}"
                      onChange={(e) =>
                        setGrant(i, { database: e.target.value || "*" })
                      }
                    />
                    <datalist id={"grant-dbs-" + i}>
                      {dbs.map((d) => (
                        <option key={d.name} value={d.name} />
                      ))}
                    </datalist>
                    <select
                      aria-label={t("userForm.level")}
                      value={g.level}
                      onChange={(e) =>
                        setGrant(i, {
                          level: e.target.value as Grant["level"],
                        })
                      }
                    >
                      <option value="view">{t("level.view")}</option>
                      <option value="operate">{t("level.operate")}</option>
                    </select>
                    <button
                      type="button"
                      className="icon-btn danger"
                      title={t("userForm.removeGrant")}
                      onClick={() =>
                        setGrants((gs) => gs.filter((_, j) => j !== i))
                      }
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                );
              })}
              <button
                type="button"
                className="secondary sm"
                onClick={() =>
                  setGrants((gs) => [
                    ...gs,
                    {
                      connection_id: profiles[0]?.id ?? "*",
                      database: "*",
                      level: "view",
                    },
                  ])
                }
              >
                <Plus size={14} /> {t("userForm.addGrant")}
              </button>
            </div>
          </>
        )}
      </fieldset>
      <div className="modal-actions">
        <button type="button" className="secondary" onClick={cancel}>
          {t("common.cancel")}
        </button>
        <button className="primary" disabled={working}>
          {working ? (
            <LoaderCircle size={16} className="spin" />
          ) : (
            <Check size={16} />
          )}
          {user ? t("common.saveChanges") : t("userForm.create")}
        </button>
      </div>
    </form>
  );
}

/** Language picker; the choice is kept in localStorage. */
export function LanguageSelect() {
  const lang = useLang();
  return (
    <select
      className="lang-select"
      aria-label={t("common.language")}
      value={lang}
      onChange={(e) => setLang(e.target.value as Lang)}
    >
      {languages.map(([code, name]) => (
        <option key={code} value={code}>
          {name}
        </option>
      ))}
    </select>
  );
}

// --------------------------------------------------------- notifications --

export const notifierTypes: NotifierType[] = [
  "telegram",
  "slack",
  "discord",
  "webhook",
  "email",
];
export const notifierTypeName = (type: string) =>
  (
    ({
      telegram: "Telegram",
      slack: "Slack",
      discord: "Discord",
      webhook: "Webhook",
      email: "Email (SMTP)",
    }) as Record<string, string>
  )[type] || type;
const notifierEvents: NotifierEvent[] = [
  "failure",
  "stale",
  "overdue",
  "backup_size",
  "connection",
  "destination",
  "disk",
  "restore",
  "security",
  "success",
];

/** Settings of each channel type: [key, label, secret?, placeholder]. */
function notifierFields(
  type: NotifierType,
): [string, string, boolean, string?][] {
  switch (type) {
    case "telegram":
      return [
        ["bot_token", t("notifyForm.botToken"), true, "123456789:AA…"],
        ["chat_id", t("notifyForm.chatId"), false, "-1001234567890"],
      ];
    case "slack":
      return [
        [
          "webhook_url",
          t("notifyForm.webhookUrl"),
          true,
          "https://hooks.slack.com/services/…",
        ],
      ];
    case "discord":
      return [
        [
          "webhook_url",
          t("notifyForm.webhookUrl"),
          true,
          "https://discord.com/api/webhooks/…",
        ],
      ];
    case "webhook":
      return [
        ["url", "URL", true, "https://example.com/hooks/backup"],
        ["secret", t("notifyForm.hmacSecret"), true, t("notifyForm.optional")],
      ];
    case "email":
      return [
        ["host", t("notifyForm.smtpHost"), false, "smtp.gmail.com"],
        ["port", "Port", false, "587"],
        ["username", t("notifyForm.smtpUser"), false, t("notifyForm.optional")],
        ["password", t("notifyForm.smtpPassword"), true, ""],
        ["from", t("notifyForm.from"), false, "DB Backup <backup@example.com>"],
        ["to", t("notifyForm.to"), false, "ops@example.com, dba@example.com"],
      ];
  }
}

export function NotifierForm({
  notifier,
  saved,
  setBusy,
}: {
  notifier: Notifier | null;
  saved: () => Promise<void>;
  setBusy: (busy: boolean) => void;
}) {
  const [form, setForm] = useState({
    name: notifier?.name || "",
    type: notifier?.type || ("telegram" as NotifierType),
    config: { ...(notifier?.config || {}) } as Record<string, string>,
    // New channels get everything except the noisy "success".
    events: notifier?.events || notifierEvents.filter((e) => e !== "success"),
    lang: notifier?.lang || getLang(),
    enabled: notifier?.enabled ?? true,
  });
  const [working, setWorking] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const sameType = notifier?.type === form.type;
  const hasSecret = (k: string) =>
    sameType && !!notifier?.secrets_set.includes(k);
  function update(patch: Partial<typeof form>) {
    setForm((f) => ({ ...f, ...patch }));
    setMessage("");
    setError("");
  }
  const setConfig = (k: string, v: string) =>
    update({ config: { ...form.config, [k]: v } });
  async function submit(test: boolean) {
    setWorking(true);
    setBusy(true);
    setMessage("");
    setError("");
    const body = { ...form, id: notifier?.id };
    try {
      if (test) {
        await api("/notifiers/test", "POST", body);
        setMessage(t("notifyForm.testOk"));
      } else {
        delete body.id;
        await api(
          notifier ? "/notifiers/" + notifier.id : "/notifiers",
          notifier ? "PUT" : "POST",
          body,
        );
        await saved();
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setWorking(false);
      setBusy(false);
    }
  }
  const fields = notifierFields(form.type);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void submit(false);
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      {message && (
        <div className="notice success" role="status">
          <CheckCircle2 size={16} />
          {message}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <label>
          {t("notifyForm.name")}
          <input
            autoFocus
            value={form.name}
            maxLength={100}
            placeholder={t("notifyForm.namePlaceholder")}
            onChange={(e) => update({ name: e.target.value })}
            required
          />
        </label>
        <div className="preset-row" role="tablist">
          {notifierTypes.map((k) => (
            <button
              key={k}
              type="button"
              role="tab"
              aria-selected={form.type === k}
              className={"chip" + (form.type === k ? " on" : "")}
              onClick={() =>
                update({
                  type: k,
                  config: notifier?.type === k ? { ...notifier.config } : {},
                })
              }
            >
              {notifierTypeName(k)}
            </button>
          ))}
        </div>
        <div className="form-note steps">
          {tx(("notifyForm.help." + form.type) as Key)}
        </div>
        {form.type === "email" && (
          <label>
            {t("notifyForm.security")}
            <select
              value={form.config.security || "starttls"}
              onChange={(e) => setConfig("security", e.target.value)}
            >
              <option value="starttls">STARTTLS (587)</option>
              <option value="tls">SSL/TLS (465)</option>
              <option value="none">{t("notifyForm.noTls")}</option>
            </select>
          </label>
        )}
        {fields.map(([key, label, secret, placeholder]) => (
          <label key={form.type + key}>
            {label}
            <input
              type={
                secret && key !== "url" && key !== "webhook_url"
                  ? "password"
                  : "text"
              }
              // Keep password managers from filling the sign-in user/password.
              autoComplete={secret ? "new-password" : "off"}
              data-1p-ignore
              data-lpignore="true"
              name={"notifier-" + key}
              spellCheck={false}
              value={form.config[key] || ""}
              placeholder={hasSecret(key) ? t("common.keepBlank") : placeholder}
              onChange={(e) => setConfig(key, e.target.value)}
              required={
                secret
                  ? !hasSecret(key) &&
                    key !== "secret" &&
                    !(key === "password" && !form.config.username)
                  : !["username", "port"].includes(key)
              }
            />
          </label>
        ))}
        <h3 className="form-section">{t("notifyForm.events")}</h3>
        <div className="event-list">
          {notifierEvents.map((ev) => (
            <label className="checkbox" key={ev}>
              <input
                type="checkbox"
                checked={form.events.includes(ev)}
                onChange={(e) =>
                  update({
                    events: e.target.checked
                      ? [...form.events, ev]
                      : form.events.filter((x) => x !== ev),
                  })
                }
              />
              <span>
                <strong>{t(("notify.event." + ev) as Key)}</strong>
                <br />
                <small className="muted">
                  {t(("notify.eventHint." + ev) as Key)}
                </small>
              </span>
            </label>
          ))}
        </div>
        <label>
          {t("notifyForm.lang")}
          <select
            value={form.lang}
            onChange={(e) =>
              update({ lang: e.target.value as Notifier["lang"] })
            }
          >
            {languages.map(([code, name]) => (
              <option key={code} value={code}>
                {name}
              </option>
            ))}
          </select>
        </label>
        <div className="modal-actions">
          <button
            type="button"
            className="secondary"
            onClick={() => void submit(true)}
            disabled={working}
          >
            {working ? (
              <LoaderCircle size={16} className="spin" />
            ) : (
              <RefreshCw size={16} />
            )}
            {t("notifyForm.test")}
          </button>
          <button
            className="primary"
            disabled={working || form.events.length === 0}
          >
            <Check size={16} />
            {notifier ? t("common.saveChanges") : t("notify.add")}
          </button>
        </div>
      </fieldset>
    </form>
  );
}

// --------------------------------------------------------------- branding --

/** The configured logo, or the default database icon. */
export function BrandLogo({
  branding,
  size = 17,
}: {
  branding: Branding;
  size?: number;
}) {
  return branding.logo ? (
    <img className="logo img" src={logoURL(branding)} alt="" />
  ) : (
    <span className="logo">
      <Database size={size} />
    </span>
  );
}

export function BrandingForm({
  branding,
  saved,
  cancel,
  setBusy,
}: {
  branding: Branding;
  saved: () => Promise<void>;
  cancel: () => void;
  setBusy: (busy: boolean) => void;
}) {
  const [name, setName] = useState(branding.name);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState("");
  const [removeLogo, setRemoveLogo] = useState(false);
  const [working, setWorking] = useState(false);
  const [error, setError] = useState("");
  // A data: URL, since the Content-Security-Policy does not allow blob: images.
  useEffect(() => {
    setPreview("");
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => setPreview(String(reader.result));
    reader.readAsDataURL(file);
    return () => reader.abort();
  }, [file]);
  const shown =
    preview || (branding.logo && !removeLogo ? logoURL(branding) : "");
  async function submit() {
    if (file && file.size > 256 * 1024) {
      setError(t("branding.tooLarge"));
      return;
    }
    setWorking(true);
    setBusy(true);
    setError("");
    try {
      if (name.trim() !== branding.name)
        await api("/settings/branding", "PUT", { name });
      if (file) {
        const res = await fetch("/api/settings/branding/logo", {
          method: "PUT",
          headers: {
            "X-Requested-With": "DBBackupManager",
            "Content-Type": "application/octet-stream",
          },
          body: file,
        });
        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(translateServer(data.error) || t("error.request"));
        }
      } else if (removeLogo && branding.logo) {
        await api("/settings/branding/logo", "DELETE");
      }
      await saved();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setWorking(false);
      setBusy(false);
    }
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void submit();
      }}
    >
      {error && (
        <div className="alert" role="alert">
          {error}
        </div>
      )}
      <fieldset disabled={working} className="form-fields">
        <label>
          {t("branding.name")}
          <input
            autoFocus
            value={name}
            maxLength={60}
            placeholder={branding.default_name}
            onChange={(e) => setName(e.target.value)}
          />
          <small>
            {t("branding.nameHint", { name: branding.default_name })}
          </small>
        </label>
        <div className="brand-preview">
          {shown ? (
            <img className="logo img" src={shown} alt="" />
          ) : (
            <span className="logo">
              <Database size={20} />
            </span>
          )}
          <strong>{name.trim() || branding.default_name}</strong>
        </div>
        <label className="file-drop">
          <ImageUp size={22} />
          <span>{file ? file.name : t("branding.pick")}</span>
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml"
            onChange={(e) => {
              setFile(e.target.files?.[0] || null);
              setRemoveLogo(false);
              setError("");
            }}
          />
        </label>
        <small className="hint">{t("branding.logoHint")}</small>
        {(branding.logo || file) && !removeLogo && (
          <button
            type="button"
            className="secondary sm"
            onClick={() => {
              setFile(null);
              setRemoveLogo(true);
            }}
          >
            <Trash2 size={14} /> {t("branding.removeLogo")}
          </button>
        )}
        <div className="modal-actions">
          <button type="button" className="secondary" onClick={cancel}>
            {t("common.cancel")}
          </button>
          <button className="primary" disabled={working}>
            {working ? (
              <LoaderCircle size={16} className="spin" />
            ) : (
              <Check size={16} />
            )}
            {t("common.saveChanges")}
          </button>
        </div>
      </fieldset>
    </form>
  );
}
