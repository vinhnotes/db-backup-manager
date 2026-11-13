import { useEffect, useRef, useState, FormEvent, ReactNode } from "react";
import {
  Activity,
  AlertCircle,
  Archive,
  ArrowLeft,
  ArrowRight,
  ArrowUpFromLine,
  Bell,
  CalendarClock,
  Check,
  CircleHelp,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Cloud,
  CloudDownload,
  CloudUpload,
  Database,
  FlaskConical,
  Home,
  Languages,
  LayoutDashboard,
  LoaderCircle,
  Menu as MenuIcon,
  Plus,
  RefreshCw,
  Search,
  Server,
  Globe,
  Settings,
  ShieldCheck,
  Trash2,
  UserPlus,
  Users,
  X,
} from "lucide-react";
import {
  ago,
  api,
  Audit,
  Backup,
  Branding,
  can,
  canConnection,
  connKey,
  ConnectionProfile,
  date,
  DB,
  Destination,
  Job,
  Me,
  Notifier,
  Offsite,
  Probe,
  providerName,
  RemoteFile,
  Schedule,
  size,
  useRoute,
  User,
  ZoneTab,
  initials,
  dbVersion,
  endpoint,
  engineOf,
  engines,
} from "./lib";
import {
  AuditTable,
  BackupTable,
  BrandingForm,
  BrandLogo,
  ConnAvatar,
  ConnectionForm,
  DestinationForm,
  Empty,
  GrantList,
  JobTable,
  Menu,
  Modal,
  NotifierForm,
  notifierTypeName,
  PasswordForm,
  roleLabel,
  UserForm,
  UserMenu,
  ProbeBadge,
  ScheduleTable,
  Stat,
  LanguageSelect,
} from "./components";
import { HelpPage } from "./help";
import { Key, t, translateServer, tx, useLang } from "./i18n";

const timezones: string[] = (() => {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return [];
  }
})();

type ModalKind =
  | "backup"
  | "restore"
  | "schedule"
  | "upload"
  | "delete"
  | "connection"
  | "deleteConnection"
  | "destination"
  | "deleteDestination"
  | "toCloud"
  | "cloudFiles"
  | "profile"
  | "user"
  | "deleteUser"
  | "timezone"
  | "verify"
  | "branding"
  | "notifier"
  | "deleteNotifier";

const zoneNav: [ZoneTab, Key, typeof Database][] = [
  ["overview", "nav.overview", LayoutDashboard],
  ["databases", "nav.databases", Database],
  ["backups", "nav.backups", Archive],
  ["schedules", "nav.schedules", CalendarClock],
  ["activity", "nav.activity", Activity],
  ["settings", "nav.settings", Settings],
];

export default function App() {
  useLang(); // re-render everything when the language changes
  const [route, go] = useRoute();
  const [me, setMe] = useState<Me | null>(null),
    [checking, setChecking] = useState(true),
    [loaded, setLoaded] = useState(false),
    [backups, setBackups] = useState<Backup[]>([]),
    [jobs, setJobs] = useState<Job[]>([]),
    [audit, setAudit] = useState<Audit[]>([]),
    [schedules, setSchedules] = useState<Schedule[]>([]),
    [profiles, setProfiles] = useState<ConnectionProfile[]>([]),
    [probes, setProbes] = useState<Record<string, Probe>>({}),
    [destinations, setDestinations] = useState<Destination[]>([]),
    [offsite, setOffsite] = useState<Offsite[]>([]),
    [users, setUsers] = useState<User[]>([]),
    [notifiers, setNotifiers] = useState<Notifier[]>([]),
    [branding, setBranding] = useState<Branding>({
      name: "DB Backup Manager",
      default_name: "DB Backup Manager",
      logo: false,
      version: "",
    }),
    [editingNotifier, setEditingNotifier] = useState<Notifier | null>(null),
    [verifyServer, setVerifyServer] = useState(""),
    [scheduleVerify, setScheduleVerify] = useState(false),
    [editingSchedule, setEditingSchedule] = useState<Schedule | null>(null),
    [scheduleVerifyConn, setScheduleVerifyConn] = useState(""),
    [editingUser, setEditingUser] = useState<User | null>(null),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false),
    // list filters
    [connQuery, setConnQuery] = useState(""),
    [connFilter, setConnFilter] = useState<"all" | "ok" | "error">("all"),
    [backupQuery, setBackupQuery] = useState(""),
    [backupServer, setBackupServer] = useState("all"),
    [backupDB, setBackupDB] = useState("all"),
    // chrome
    [navOpen, setNavOpen] = useState(false),
    [switcherOpen, setSwitcherOpen] = useState(false),
    // modals
    [modal, setModal] = useState<ModalKind | null>(null),
    [modalConn, setModalConn] = useState(""),
    [selected, setSelected] = useState<Backup | null>(null),
    [editingConnection, setEditingConnection] =
      useState<ConnectionProfile | null>(null),
    [deletingConnection, setDeletingConnection] =
      useState<ConnectionProfile | null>(null),
    [editingDestination, setEditingDestination] = useState<Destination | null>(
      null,
    ),
    [destinationChoice, setDestinationChoice] = useState(""),
    [remote, setRemote] = useState<{
      state: "loading" | "ok" | "error";
      files: RemoteFile[];
      error?: string;
    }>({ state: "loading", files: [] }),
    [remotePick, setRemotePick] = useState<RemoteFile | null>(null),
    [pullSecret, setPullSecret] = useState(""),
    [database, setDatabase] = useState("*"),
    [restoreServer, setRestoreServer] = useState(""),
    [target, setTarget] = useState(""),
    [confirmation, setConfirmation] = useState(""),
    [trusted, setTrusted] = useState(false),
    [cron, setCron] = useState("0 2 * * *"),
    [days, setDays] = useState(14),
    [tzInput, setTzInput] = useState(""),
    [file, setFile] = useState<File | null>(null),
    [username, setUsername] = useState("admin"),
    [password, setPassword] = useState("");
  const admin = me?.role === "admin";
  // Permission helpers; the server enforces the same rules.
  const canOp = (conn: string, db: string) => can(me, conn, db, "operate");
  const canOpConn = (conn: string) => canConnection(me, conn, "operate");
  const canOpBackup = (b: { connection_id: string; database: string }) =>
    canOp(b.connection_id, b.database);
  const viewAll = can(me, "*", "*", "view");

  async function refresh() {
    try {
      const [b, j, s, a, c, d, o] = await Promise.all([
        api<Backup[]>("/backups"),
        api<Job[]>("/jobs"),
        api<Schedule[]>("/schedules"),
        api<Audit[]>("/audit"),
        api<ConnectionProfile[]>("/connections"),
        api<Destination[]>("/destinations"),
        api<Offsite[]>("/offsite"),
      ]);
      setDestinations(d);
      setOffsite(o);
      setBackups(b);
      setJobs(j);
      setSchedules(s);
      setAudit(a);
      setProfiles(c);
      if (admin) {
        const [u, n] = await Promise.all([
          api<User[]>("/users"),
          api<Notifier[]>("/notifiers"),
        ]);
        setUsers(u);
        setNotifiers(n);
      }
      setLoaded(true);
    } catch (e) {
      setError((e as Error).message);
    }
  }

  // Each connection is probed independently; a sequence number per id drops
  // stale responses when a newer probe for the same connection was started.
  const probeSeq = useRef<Record<string, number>>({});
  async function probe(id: string, reset = false) {
    const seq = (probeSeq.current[id] || 0) + 1;
    probeSeq.current[id] = seq;
    setProbes((p) =>
      p[id] && !reset ? p : { ...p, [id]: { state: "loading", dbs: [] } },
    );
    try {
      const d = await api<{ databases: DB[]; version: string }>(
        "/databases?connection_id=" + encodeURIComponent(id),
      );
      if (probeSeq.current[id] !== seq) return;
      setProbes((p) => ({
        ...p,
        [id]: { state: "ok", version: d.version, dbs: d.databases },
      }));
    } catch (e) {
      if (probeSeq.current[id] !== seq) return;
      setProbes((p) => ({
        ...p,
        [id]: { state: "error", dbs: [], error: (e as Error).message },
      }));
    }
  }

  const loadBranding = () =>
    api<Branding>("/branding")
      .then(setBranding)
      .catch(() => {});
  useEffect(() => {
    void loadBranding();
  }, []);
  // Tab title and favicon follow the branding.
  useEffect(() => {
    document.title = branding.name;
    // The server serves the logo, or a default icon, at /api/branding/favicon;
    // the version makes the browser fetch it again after a change.
    let icon = document.querySelector<HTMLLinkElement>("link[rel='icon']");
    if (!icon) {
      icon = document.createElement("link");
      icon.rel = "icon";
      document.head.appendChild(icon);
    }
    icon.href =
      "/api/branding/favicon?v=" + encodeURIComponent(branding.version);
  }, [branding]);
  useEffect(() => {
    api<Me>("/me")
      .then(setMe)
      .catch(() => {})
      .finally(() => setChecking(false));
  }, []);
  useEffect(() => {
    if (!me) return;
    void refresh();
    const t = setInterval(() => void refresh(), 5000);
    return () => clearInterval(t);
  }, [me]);
  const profileIds = JSON.stringify(profiles.map((p) => p.id));
  useEffect(() => {
    if (!me) return;
    const ids: string[] = JSON.parse(profileIds);
    ids.forEach((id) => void probe(id));
    const t = setInterval(() => ids.forEach((id) => void probe(id)), 30000);
    return () => clearInterval(t);
  }, [me, profileIds]);
  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => setNotice(""), 6000);
    return () => clearTimeout(t);
  }, [notice]);
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !busy) {
        setModal(null);
        setSwitcherOpen(false);
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [busy]);
  useEffect(() => {
    setNavOpen(false);
    setSwitcherOpen(false);
    setError("");
  }, [route.view, route.view === "zone" ? route.id : ""]);

  async function action(fn: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    try {
      await fn();
      setModal(null);
      setNotice(message);
      await refresh();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function login(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/login", "POST", { username, password });
      setNotice("");
      setMe(await api<Me>("/me"));
      setPassword("");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const zone =
    route.view === "zone" ? profiles.find((p) => p.id === route.id) : undefined;
  const defaultConn = zone
    ? zone.id
    : (profiles.find((p) => canOpConn(p.id))?.id ?? profiles[0]?.id ?? "");
  const operable = profiles.filter((p) => canOpConn(p.id));
  const connectionName = (id: string) =>
    profiles.find((p) => p.id === id)?.name ||
    (id ? t("conn.deleted") : t("conn.env"));
  const restoreTargets = profiles.filter(
    (p) => p.restore_enabled && canOp(p.id, "*"),
  );
  const restoreAvailable = restoreTargets.length > 0;
  const destinationName = (id: string) =>
    destinations.find((d) => d.id === id)?.name || t("dest.deleted");
  const cloudCopies = (b: Backup) =>
    offsite
      .filter((o) => o.backup_id === b.id)
      .map((o) => destinationName(o.destination_id));
  const toCloud = destinations.length ? openToCloud : undefined;

  function openModal(kind: ModalKind, conn = defaultConn) {
    if (
      ["backup", "schedule", "upload"].includes(kind) &&
      !canOpConn(conn) &&
      operable[0]
    )
      conn = operable[0].id;
    setModalConn(conn);
    setError("");
    setModal(kind);
  }
  function openBackup(conn = defaultConn, name = "*") {
    setDatabase(name);
    setDestinationChoice("");
    openModal("backup", conn);
  }
  function openSchedule(conn = defaultConn) {
    setEditingSchedule(null);
    setDatabase("*");
    setDestinationChoice(destinations[0]?.id ?? "");
    setScheduleVerify(false);
    setScheduleVerifyConn("");
    openModal("schedule", conn);
  }
  function openEditSchedule(s: Schedule) {
    setEditingSchedule(s);
    setDatabase(s.database);
    setCron(s.cron);
    setDays(s.retention_days);
    setDestinationChoice(s.destination_id);
    setScheduleVerify(!!s.verify);
    setScheduleVerifyConn(s.verify_connection_id || "");
    void probe(s.connection_id);
    openModal("schedule", s.connection_id);
  }
  function openVerify(b: Backup) {
    setSelected(b);
    const targets = restoreTargets.filter(
      (p) => engineOf(p.engine) === engineOf(b.engine),
    );
    setVerifyServer(
      targets.find((p) => p.id === b.connection_id)?.id ?? targets[0]?.id ?? "",
    );
    openModal("verify");
  }
  function openNotifier(n: Notifier | null = null) {
    setEditingNotifier(n);
    openModal("notifier");
  }
  // Secret settings are left out: the server keeps the stored ones.
  const toggleNotifier = (n: Notifier) =>
    action(
      () =>
        api("/notifiers/" + n.id, "PUT", {
          name: n.name,
          type: n.type,
          config: n.config,
          events: n.events,
          lang: n.lang,
          enabled: !n.enabled,
        }),
      n.enabled ? t("notice.notifierPaused") : t("notice.notifierResumed"),
    );
  function openDeleteNotifier(n: Notifier) {
    setEditingNotifier(n);
    openModal("deleteNotifier");
  }
  function openDestination(d: Destination | null = null) {
    setEditingDestination(d);
    openModal("destination");
  }
  function openDeleteDestination(d: Destination) {
    setEditingDestination(d);
    openModal("deleteDestination");
  }
  function openToCloud(b: Backup) {
    setSelected(b);
    setDestinationChoice(destinations[0]?.id ?? "");
    openModal("toCloud");
  }
  async function openCloudFiles(d: Destination) {
    setEditingDestination(d);
    setRemotePick(null);
    setPullSecret("");
    setRemote({ state: "loading", files: [] });
    openModal("cloudFiles", profiles[0]?.id ?? "");
    try {
      const files = await api<RemoteFile[]>("/destinations/" + d.id + "/files");
      setRemote({ state: "ok", files });
    } catch (e) {
      setRemote({ state: "error", files: [], error: (e as Error).message });
    }
  }
  function openUpload(conn = defaultConn) {
    setDatabase("");
    setFile(null);
    openModal("upload", conn);
  }
  function openTimezone() {
    setError("");
    setTzInput(me!.timezone);
    setModal("timezone");
  }
  function openConnection(profile: ConnectionProfile | null = null) {
    setEditingConnection(profile);
    openModal("connection");
  }
  function openDeleteConnection(p: ConnectionProfile) {
    setDeletingConnection(p);
    openModal("deleteConnection");
  }
  function openRestore(b: Backup) {
    setSelected(b);
    // A backup can only be restored into a server of the same engine.
    const targets = restoreTargets.filter(
      (p) => engineOf(p.engine) === engineOf(b.engine),
    );
    setRestoreServer(
      targets.find((p) => p.id === b.connection_id)?.id ?? targets[0]?.id ?? "",
    );
    setTarget(b.database + "_restored");
    setConfirmation("");
    setTrusted(false);
    openModal("restore");
  }
  function openUser(u: User | null = null) {
    setEditingUser(u);
    openModal("user");
  }
  function openDeleteUser(u: User) {
    setEditingUser(u);
    openModal("deleteUser");
  }
  const logout = () =>
    action(async () => {
      await api("/logout", "POST");
      setMe(null);
      setUsers([]);
      go("/");
    }, t("notice.loggedOut"));
  function openDeleteBackup(b: Backup) {
    setSelected(b);
    openModal("delete");
  }
  const toggleSchedule = (s: Schedule) =>
    action(
      () =>
        api("/schedules", "POST", {
          id: s.id,
          connection_id: s.connection_id,
          database: s.database,
          cron: s.cron,
          retention_days: s.retention_days,
          enabled: !s.enabled,
          destination_id: s.destination_id,
          verify: !!s.verify,
          verify_connection_id: s.verify_connection_id || "",
        }),
      t("notice.scheduleUpdated"),
    );
  const deleteSchedule = (s: Schedule) =>
    action(
      () => api("/schedules/" + s.id, "DELETE"),
      t("notice.scheduleDeleted"),
    );

  const active = jobs.filter((j) => ["queued", "running"].includes(j.status));

  if (checking)
    return (
      <div className="loading">
        <LoaderCircle className="spin" /> {t("common.loading")}
      </div>
    );

  if (!me)
    return (
      <div className="login-wrap">
        <div className="login-brand">
          <BrandLogo branding={branding} size={20} />
          {branding.name}
        </div>
        <form className="login-card" onSubmit={login}>
          <h1>{t("login.title")}</h1>
          <p>{t("login.subtitle")}</p>
          {error && <div className="alert">{error}</div>}
          <label>
            {t("login.username")}
            <input
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </label>
          <label>
            {t("login.password")}
            <input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </label>
          <button className="primary wide" disabled={busy}>
            {busy ? (
              <LoaderCircle size={18} className="spin" />
            ) : (
              t("login.submit")
            )}
          </button>
        </form>
        <div className="login-lang">
          <Languages size={15} />
          <LanguageSelect />
        </div>
      </div>
    );

  // ---------------------------------------------------------------- pages --

  const pageHead = (title: string, sub?: ReactNode, actions?: ReactNode) => (
    <div className="page-head">
      <div>
        <h1>{title}</h1>
        {sub && <p>{sub}</p>}
      </div>
      {actions && <div className="head-actions">{actions}</div>}
    </div>
  );

  function HomePage() {
    const q = connQuery.trim().toLowerCase();
    const list = profiles.filter((p) => {
      const st = probes[p.id]?.state;
      if (connFilter === "ok" && st !== "ok") return false;
      if (connFilter === "error" && st !== "error") return false;
      return (
        !q ||
        p.name.toLowerCase().includes(q) ||
        p.host.toLowerCase().includes(q)
      );
    });
    const okCount = profiles.filter((p) => probes[p.id]?.state === "ok").length;
    const errCount = profiles.filter(
      (p) => probes[p.id]?.state === "error",
    ).length;
    return (
      <>
        {pageHead(
          t("nav.connections"),
          t("home.subtitle"),
          admin && (
            <button className="primary" onClick={() => openConnection()}>
              <Plus size={16} /> {t("conn.add")}
            </button>
          ),
        )}
        <div className="toolbar">
          <div className="search">
            <Search size={16} />
            <input
              placeholder={t("home.search")}
              value={connQuery}
              onChange={(e) => setConnQuery(e.target.value)}
            />
          </div>
          <div className="segmented" role="tablist">
            {(
              [
                ["all", t("home.filterAll", { n: profiles.length })],
                ["ok", t("home.filterOk", { n: okCount })],
                ["error", t("home.filterError", { n: errCount })],
              ] as const
            ).map(([k, text]) => (
              <button
                key={k}
                role="tab"
                aria-selected={connFilter === k}
                className={connFilter === k ? "on" : ""}
                onClick={() => setConnFilter(k)}
              >
                {text}
              </button>
            ))}
          </div>
        </div>
        {list.length ? (
          <div className="site-list">
            {list.map((p) => {
              const pr = probes[p.id];
              const pb = backups.filter((b) => b.connection_id === p.id);
              const sc = schedules.filter(
                (s) => s.connection_id === p.id && s.enabled,
              ).length;
              const open = () => go("/c/" + connKey(p.id));
              return (
                <div
                  className="site-row"
                  key={p.id || "_env"}
                  role="link"
                  tabIndex={0}
                  onClick={open}
                  onKeyDown={(e) => e.key === "Enter" && open()}
                >
                  <div className="site-name">
                    <ConnAvatar name={p.name} />
                    <div>
                      <a
                        href={"#/c/" + connKey(p.id)}
                        onClick={(e) => e.stopPropagation()}
                      >
                        {p.name}
                      </a>
                      <small>
                        {engines[engineOf(p.engine)].label} · {endpoint(p)}
                        {p.username && <> · {p.username}</>}
                      </small>
                    </div>
                  </div>
                  <div className="site-status">
                    <ProbeBadge probe={pr} />
                  </div>
                  <div className="site-meta">
                    <span>
                      <small>{t("common.version")}</small>
                      {pr?.state === "ok"
                        ? dbVersion(p.engine, pr.version)
                        : "—"}
                    </span>
                    <span>
                      <small>{t("common.databases")}</small>
                      {pr?.state === "ok" ? pr.dbs.length : "—"}
                    </span>
                    <span>
                      <small>{t("common.backups")}</small>
                      {pb.length} · {size(pb.reduce((a, b) => a + b.size, 0))}
                    </span>
                    <span>
                      <small>{t("common.lastBackup")}</small>
                      {pb[0] ? ago(pb[0].created_at) : t("common.none")}
                    </span>
                    <span>
                      <small>{t("common.schedules")}</small>
                      {sc ? t("home.schedulesOn", { n: sc }) : t("common.no")}
                    </span>
                  </div>
                  <div
                    className="site-actions"
                    onClick={(e) => e.stopPropagation()}
                  >
                    <Menu
                      items={[
                        [t("menu.open"), open],
                        ...(canOpConn(p.id)
                          ? ([
                              [t("action.backupNow"), () => openBackup(p.id)],
                              [
                                t("action.addSchedule"),
                                () => openSchedule(p.id),
                              ],
                            ] as [string, () => void][])
                          : []),
                        [t("action.recheck"), () => void probe(p.id, true)],
                        ...(admin
                          ? ([
                              ...(p.id
                                ? [
                                    [t("conn.edit"), () => openConnection(p)],
                                    [
                                      t("conn.delete"),
                                      () => openDeleteConnection(p),
                                      true,
                                    ],
                                  ]
                                : []),
                            ] as [string, () => void, boolean?][])
                          : []),
                      ]}
                    />
                    <ChevronRight size={18} className="chev" />
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="card">
            <Empty
              icon={<Server size={28} />}
              title={profiles.length ? t("home.noResults") : t("home.empty")}
              text={
                profiles.length ? t("home.noResultsText") : t("home.emptyText")
              }
              action={
                admin && !profiles.length ? (
                  <button className="primary" onClick={() => openConnection()}>
                    <Plus size={16} /> {t("conn.add")}
                  </button>
                ) : undefined
              }
            />
          </div>
        )}
      </>
    );
  }

  function BackupsPanel({ connId }: { connId?: string }) {
    const scoped = connId !== undefined;
    const server = scoped ? connId : backupServer;
    const inServer = backups.filter(
      (b) => server === "all" || b.connection_id === server,
    );
    const dbNames = [...new Set(inServer.map((b) => b.database))].sort();
    const q = backupQuery.toLowerCase();
    const list = inServer.filter(
      (b) =>
        (backupDB === "all" || b.database === backupDB) &&
        b.database.toLowerCase().includes(q),
    );
    return (
      <div className="card">
        <div className="card-toolbar">
          <div className="search">
            <Search size={16} />
            <input
              placeholder={t("backups.search")}
              value={backupQuery}
              onChange={(e) => setBackupQuery(e.target.value)}
            />
          </div>
          {!scoped && (
            <select
              value={backupServer}
              onChange={(e) => {
                setBackupServer(e.target.value);
                setBackupDB("all");
              }}
              aria-label={t("common.connection")}
            >
              <option value="all">{t("common.allConnections")}</option>
              {[
                ...new Set([
                  ...profiles.map((p) => p.id),
                  ...backups.map((b) => b.connection_id),
                ]),
              ].map((id) => (
                <option key={id || "_env"} value={id}>
                  {connectionName(id)}
                  {id && !profiles.some((p) => p.id === id)
                    ? ` (${id.slice(0, 8)})`
                    : ""}
                </option>
              ))}
            </select>
          )}
          <select
            value={backupDB}
            onChange={(e) => setBackupDB(e.target.value)}
            aria-label={t("common.database")}
          >
            <option value="all">{t("common.allDatabases")}</option>
            {dbNames.map((n) => (
              <option key={n}>{n}</option>
            ))}
          </select>
          <span className="muted">
            {t("backups.count", {
              n: list.length,
              size: size(list.reduce((s, b) => s + b.size, 0)),
            })}
          </span>
        </div>
        {!restoreAvailable && admin && (
          <div className="notice">
            <AlertCircle size={16} /> {t("backups.restoreHint")}
          </div>
        )}
        {list.length ? (
          <BackupTable
            key={server + "|" + backupDB + "|" + q}
            backups={list}
            canOperate={canOpBackup}
            restoreEnabled={restoreAvailable}
            connectionName={scoped ? undefined : connectionName}
            restore={openRestore}
            remove={openDeleteBackup}
            cloudCopies={cloudCopies}
            toCloud={toCloud}
            verify={openVerify}
          />
        ) : (
          <Empty
            icon={<Archive size={28} />}
            title={
              backupQuery || backupDB !== "all" || server !== "all"
                ? t("backups.noMatch")
                : t("backups.empty")
            }
            text={
              operable.length
                ? t("backups.emptyText")
                : t("backups.emptyNoAccess")
            }
          />
        )}
      </div>
    );
  }

  function SchedulesPanel({ connId }: { connId?: string }) {
    const list =
      connId === undefined
        ? schedules
        : schedules.filter((s) => s.connection_id === connId);
    return (
      <>
        <div className="notice">
          <CalendarClock size={16} />
          <span>{tx("schedules.tzNotice", { tz: me!.timezone })}</span>
        </div>
        <div className="card">
          {list.length ? (
            <ScheduleTable
              schedules={list}
              canOperate={canOpBackup}
              connectionName={connId === undefined ? connectionName : undefined}
              destinationName={destinationName}
              toggle={toggleSchedule}
              edit={openEditSchedule}
              remove={deleteSchedule}
            />
          ) : (
            <Empty
              icon={<CalendarClock size={28} />}
              title={t("schedules.empty")}
              text={t("schedules.emptyText")}
              action={
                (connId === undefined ? operable.length : canOpConn(connId)) ? (
                  <button
                    className="primary"
                    onClick={() => openSchedule(connId ?? defaultConn)}
                  >
                    <Plus size={16} /> {t("action.addSchedule")}
                  </button>
                ) : undefined
              }
            />
          )}
        </div>
      </>
    );
  }

  function JobsCard({ connId, limit }: { connId?: string; limit?: number }) {
    let list =
      connId === undefined
        ? jobs
        : jobs.filter((j) => j.connection_id === connId);
    if (limit) list = list.slice(0, limit);
    return list.length ? (
      <JobTable
        jobs={list}
        connectionName={connId === undefined ? connectionName : undefined}
      />
    ) : (
      <Empty
        icon={<Activity size={28} />}
        title={t("jobs.empty")}
        text={t("jobs.emptyText")}
      />
    );
  }

  function ZoneOverview({ p }: { p: ConnectionProfile }) {
    const pr = probes[p.id];
    const pb = backups.filter((b) => b.connection_id === p.id);
    const ps = schedules.filter((s) => s.connection_id === p.id);
    return (
      <div className="zone-grid">
        <div className="zone-main">
          <div className="stats">
            <Stat
              label={t("common.databases")}
              value={pr?.state === "ok" ? String(pr.dbs.length) : "—"}
              sub={
                pr?.state === "ok"
                  ? t("overview.dataSize", {
                      size: size(pr.dbs.reduce((a, d) => a + d.size, 0)),
                    })
                  : t("overview.notConnected")
              }
            />
            <Stat
              label={t("overview.backups")}
              value={String(pb.length)}
              sub={t("overview.stored", {
                size: size(pb.reduce((a, b) => a + b.size, 0)),
              })}
            />
            <Stat
              label={t("common.lastBackup")}
              value={pb[0] ? ago(pb[0].created_at) : "—"}
              sub={pb[0] ? date(pb[0].created_at) : t("backups.empty")}
            />
            <Stat
              label={t("overview.activeSchedules")}
              value={String(ps.filter((s) => s.enabled).length)}
              sub={t("overview.scheduleCount", {
                n: ps.length,
                tz: me!.timezone,
              })}
            />
          </div>
          <div className="card">
            <div className="card-head">
              <h2>{t("overview.recentBackups")}</h2>
              <a href={"#/c/" + connKey(p.id) + "/backups"} className="link">
                {t("common.viewAll")} <ArrowRight size={14} />
              </a>
            </div>
            {pb.length ? (
              <BackupTable
                backups={pb.slice(0, 5)}
                canOperate={canOpBackup}
                restoreEnabled={restoreAvailable}
                restore={openRestore}
                remove={openDeleteBackup}
                cloudCopies={cloudCopies}
                toCloud={toCloud}
                verify={openVerify}
              />
            ) : (
              <Empty
                icon={<Archive size={28} />}
                title={t("backups.empty")}
                text={t("overview.backupsEmptyText")}
                action={
                  canOpConn(p.id) ? (
                    <button
                      className="secondary"
                      onClick={() => openBackup(p.id)}
                    >
                      <Plus size={16} /> {t("action.createBackup")}
                    </button>
                  ) : undefined
                }
              />
            )}
          </div>
          <div className="card">
            <div className="card-head">
              <h2>{t("overview.recentActivity")}</h2>
              <a href={"#/c/" + connKey(p.id) + "/activity"} className="link">
                {t("common.viewAll")} <ArrowRight size={14} />
              </a>
            </div>
            {JobsCard({ connId: p.id, limit: 5 })}
          </div>
        </div>
        <aside className="zone-rail">
          {canOpConn(p.id) && (
            <div className="card rail-card">
              <h3>{t("overview.quickActions")}</h3>
              <button
                className="primary wide"
                disabled={pr?.state !== "ok"}
                onClick={() => openBackup(p.id)}
              >
                <Plus size={16} /> {t("action.backupNow")}
              </button>
              <button
                className="secondary wide"
                onClick={() => openSchedule(p.id)}
              >
                <CalendarClock size={16} /> {t("action.addSchedule")}
              </button>
              <button
                className="secondary wide"
                onClick={() => openUpload(p.id)}
              >
                <ArrowUpFromLine size={16} /> {t("action.uploadBackup")}
              </button>
            </div>
          )}
          <div className="card rail-card">
            <h3>{t("overview.connInfo")}</h3>
            <dl className="kv">
              <dt>{t("connForm.engine")}</dt>
              <dd>{engines[engineOf(p.engine)].label}</dd>
              <dt>
                {p.engine === "files" ? t("common.folder") : t("common.host")}
              </dt>
              <dd>
                <code>{endpoint(p)}</code>
              </dd>
              {p.engine === "files" ? (
                <>
                  <dt>{t("connForm.itemName")}</dt>
                  <dd>{p.database}</dd>
                </>
              ) : (
                <>
                  <dt>{t("common.account")}</dt>
                  <dd>{p.username}</dd>
                  <dt>{t("common.database")}</dt>
                  <dd>{p.database}</dd>
                  <dt>SSL</dt>
                  <dd>{p.sslmode}</dd>
                  <dt>{t("common.version")}</dt>
                  <dd>
                    {pr?.state === "ok" ? dbVersion(p.engine, pr.version) : "—"}
                  </dd>
                </>
              )}
              <dt>{t("common.restore")}</dt>
              <dd>
                {p.restore_enabled ? (
                  <span className="ok-text">
                    <Check size={13} /> {p.restore_username}
                  </span>
                ) : (
                  t("common.notConfigured")
                )}
              </dd>
            </dl>
            <a href={"#/c/" + connKey(p.id) + "/settings"} className="link">
              {t("overview.connSettings")} <ArrowRight size={14} />
            </a>
          </div>
        </aside>
      </div>
    );
  }

  function ZoneDatabases({ p }: { p: ConnectionProfile }) {
    const pr = probes[p.id];
    if (pr?.state !== "ok")
      return (
        <div className="card">
          <Empty
            icon={<Database size={28} />}
            title={
              pr?.state === "error" ? t("dbs.loadError") : t("dbs.connecting")
            }
            text={pr?.error || t("dbs.checking")}
          />
        </div>
      );
    return (
      <div className="card">
        {pr.dbs.length ? (
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>{t("common.database")}</th>
                  <th className="num">{t("common.size")}</th>
                  <th className="num">{t("overview.backups")}</th>
                  <th>{t("common.lastBackup")}</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {pr.dbs.map((d) => {
                  const db = backups.filter(
                    (b) => b.connection_id === p.id && b.database === d.name,
                  );
                  return (
                    <tr key={d.name}>
                      <td>
                        <div className="cell-main">
                          <Database size={15} />
                          <strong>{d.name}</strong>
                        </div>
                      </td>
                      <td className="num">{size(d.size)}</td>
                      <td className="num">
                        <button
                          className="link"
                          onClick={() => {
                            setBackupDB(d.name);
                            setBackupQuery("");
                            go("/c/" + connKey(p.id) + "/backups");
                          }}
                        >
                          {db.length}
                        </button>
                      </td>
                      <td>{db[0] ? ago(db[0].created_at) : "—"}</td>
                      <td>
                        {canOp(p.id, d.name) && (
                          <div className="row-actions">
                            <button
                              className="secondary sm"
                              onClick={() => openBackup(p.id, d.name)}
                            >
                              <Plus size={14} /> {t("common.backup")}
                            </button>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            icon={<Database size={28} />}
            title={t("dbs.empty")}
            text={t("dbs.emptyText")}
          />
        )}
      </div>
    );
  }

  function ZoneSettings({ p }: { p: ConnectionProfile }) {
    const row = (title: string, text: ReactNode, control?: ReactNode) => (
      <div className="setting">
        <div>
          <h3>{title}</h3>
          <div className="setting-text">{text}</div>
        </div>
        {control && <div className="setting-control">{control}</div>}
      </div>
    );
    return (
      <div className="settings">
        <div className="card">
          {row(
            t("settings.server"),
            p.engine === "files" ? (
              tx("settings.filesText", {
                addr: <code>{endpoint(p)}</code>,
                db: p.database,
              })
            ) : (
              <>
                {tx("settings.serverText", {
                  addr: <code>{endpoint(p)}</code>,
                  user: p.username,
                  db: p.database,
                  ssl: p.sslmode,
                })}
                {" · "}
                {p.has_password
                  ? t("settings.hasPassword")
                  : t("settings.noPassword")}
              </>
            ),
            admin && (
              <button className="secondary" onClick={() => openConnection(p)}>
                {t("common.edit")}
              </button>
            ),
          )}
          {row(
            t("common.restore"),
            p.restore_enabled
              ? tx(
                  p.engine === "files"
                    ? "settings.restoreDir"
                    : "settings.restoreOn",
                  { user: p.restore_username },
                )
              : t("settings.restoreOff"),
            admin && (
              <button className="secondary" onClick={() => openConnection(p)}>
                {t("settings.configure")}
              </button>
            ),
          )}
          {row(
            t("settings.check"),
            <>
              {t("settings.currentStatus")} <ProbeBadge probe={probes[p.id]} />
              {probes[p.id]?.error && (
                <div className="error-text">{probes[p.id]?.error}</div>
              )}
            </>,
            <button
              className="secondary"
              onClick={() => void probe(p.id, true)}
            >
              <RefreshCw size={15} /> {t("action.recheck")}
            </button>,
          )}
        </div>
        {admin && (
          <div className="card danger-zone">
            {row(
              t("conn.delete"),
              t("settings.deleteText"),
              <button
                className="danger-btn"
                onClick={() => openDeleteConnection(p)}
              >
                <Trash2 size={15} /> {t("conn.delete")}
              </button>,
            )}
          </div>
        )}
      </div>
    );
  }

  function CloudPage() {
    return (
      <>
        {pageHead(
          t("nav.cloud"),
          t("cloud.subtitle"),
          admin && (
            <button className="primary" onClick={() => openDestination()}>
              <Plus size={16} /> {t("dest.add")}
            </button>
          ),
        )}
        {!me!.offsite_available && (
          <div className="alert">
            <AlertCircle size={16} />
            <div>{tx("cloud.noRclone")}</div>
          </div>
        )}
        {destinations.length ? (
          <div className="dest-list">
            {destinations.map((d) => {
              const copies = offsite.filter((o) => o.destination_id === d.id);
              const scheduled = schedules.filter(
                (s) => s.destination_id === d.id,
              ).length;
              return (
                <div className="card dest-card" key={d.id}>
                  <header>
                    <span className="dest-icon">
                      <Cloud size={18} />
                    </span>
                    <div className="dest-title">
                      <strong title={d.name}>{d.name}</strong>
                      <small title={"/" + d.remote_path}>
                        {providerName(d.type)} · <code>/{d.remote_path}</code>
                      </small>
                    </div>
                    {admin && (
                      <Menu
                        items={[
                          [t("common.edit"), () => openDestination(d)],
                          [
                            t("common.delete"),
                            () => openDeleteDestination(d),
                            true,
                          ],
                        ]}
                      />
                    )}
                  </header>
                  <dl className="kv">
                    <dt>{t("cloud.encryption")}</dt>
                    <dd>
                      <span className="ok-text">
                        <ShieldCheck size={13} />
                        {d.encryption === "recipient"
                          ? "age · public key"
                          : "age · passphrase"}
                      </span>
                    </dd>
                    <dt>{t("cloud.copies")}</dt>
                    <dd>
                      {copies.length} ·{" "}
                      {size(copies.reduce((a, o) => a + o.size, 0))}
                    </dd>
                    <dt>{t("cloud.latest")}</dt>
                    <dd>
                      {copies[0]
                        ? ago(
                            copies.reduce((a, o) =>
                              o.uploaded_at > a.uploaded_at ? o : a,
                            ).uploaded_at,
                          )
                        : t("common.none")}
                    </dd>
                    <dt>{t("cloud.keep")}</dt>
                    <dd>
                      {d.retention_days
                        ? t("cloud.keepDays", { n: d.retention_days })
                        : t("cloud.forever")}
                    </dd>
                    <dt>{t("common.schedules")}</dt>
                    <dd>
                      {scheduled
                        ? t("cloud.scheduleUse", { n: scheduled })
                        : "—"}
                    </dd>
                  </dl>
                  {viewAll && (
                    <footer>
                      <button
                        className="secondary sm"
                        onClick={() => void openCloudFiles(d)}
                      >
                        <CloudDownload size={14} /> {t("cloud.browse")}
                      </button>
                    </footer>
                  )}
                </div>
              );
            })}
          </div>
        ) : (
          <div className="card">
            <Empty
              icon={<Cloud size={28} />}
              title={t("cloud.empty")}
              text={t("cloud.emptyText")}
              action={
                admin ? (
                  <button className="primary" onClick={() => openDestination()}>
                    <Plus size={16} /> {t("dest.add")}
                  </button>
                ) : undefined
              }
            />
          </div>
        )}
        <div className="notice">
          <ShieldCheck size={16} />
          <span>{tx("cloud.decryptNote")}</span>
        </div>
      </>
    );
  }

  function ZonePage() {
    if (route.view !== "zone") return null;
    if (!zone)
      return loaded ? (
        <div className="card">
          <Empty
            icon={<Server size={28} />}
            title={t("zone.notFound")}
            text={t("zone.notFoundText")}
            action={
              <button className="secondary" onClick={() => go("/")}>
                <ArrowLeft size={16} /> {t("zone.back")}
              </button>
            }
          />
        </div>
      ) : (
        <div className="loading inline">
          <LoaderCircle className="spin" />
        </div>
      );
    const p = zone;
    const pr = probes[p.id];
    const tab = route.tab;
    const tabTitle = t(zoneNav.find(([k]) => k === tab)![1]);
    return (
      <>
        <div className="zone-head">
          <ConnAvatar name={p.name} />
          <div>
            <div className="zone-title">
              <h1>{p.name}</h1>
              <ProbeBadge probe={pr} />
            </div>
            <p>
              {endpoint(p)}
              {pr?.state === "ok" && <> · {dbVersion(p.engine, pr.version)}</>}
            </p>
          </div>
          {canOpConn(p.id) && tab !== "settings" && (
            <div className="head-actions">
              {tab === "schedules" ? (
                <button className="primary" onClick={() => openSchedule(p.id)}>
                  <Plus size={16} /> {t("action.addSchedule")}
                </button>
              ) : (
                <>
                  {tab === "backups" && (
                    <button
                      className="secondary"
                      onClick={() => openUpload(p.id)}
                    >
                      <ArrowUpFromLine size={16} /> {t("action.upload")}
                    </button>
                  )}
                  <button
                    className="primary"
                    disabled={pr?.state !== "ok"}
                    onClick={() => openBackup(p.id)}
                  >
                    <Plus size={16} /> {t("action.backupNow")}
                  </button>
                </>
              )}
            </div>
          )}
        </div>
        {pr?.state === "error" && tab !== "settings" && (
          <div className="alert">
            <AlertCircle size={16} />
            <div>
              <strong>{t("zone.pgError")}</strong> {pr.error}
            </div>
            <button className="link" onClick={() => void probe(p.id, true)}>
              {t("common.retry")}
            </button>
          </div>
        )}
        {tab !== "overview" && <h2 className="section-title">{tabTitle}</h2>}
        {tab === "overview" && ZoneOverview({ p })}
        {tab === "databases" && ZoneDatabases({ p })}
        {tab === "backups" && BackupsPanel({ connId: p.id })}
        {tab === "schedules" && SchedulesPanel({ connId: p.id })}
        {tab === "activity" && (
          <div className="card">{JobsCard({ connId: p.id })}</div>
        )}
        {tab === "settings" && ZoneSettings({ p })}
      </>
    );
  }

  function UsersPage() {
    if (!admin)
      return (
        <div className="card">
          <Empty
            icon={<ShieldCheck size={28} />}
            title={t("users.adminRequired")}
            text={t("users.adminRequiredText")}
          />
        </div>
      );
    return (
      <>
        {pageHead(
          t("nav.users"),
          t("users.subtitle"),
          <button className="primary" onClick={() => openUser()}>
            <UserPlus size={16} /> {t("users.add")}
          </button>,
        )}
        <div className="card">
          {users.length ? (
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>{t("users.user")}</th>
                    <th>{t("users.role")}</th>
                    <th>{t("users.permissions")}</th>
                    <th>{t("common.status")}</th>
                    <th>{t("users.createdAt")}</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {users.map((u) => (
                    <tr key={u.id}>
                      <td>
                        <div className="user-cell">
                          <span className="avatar">
                            {initials(u.display_name || u.username)}
                          </span>
                          <span>
                            <strong>{u.display_name || u.username}</strong>
                            <small>@{u.username}</small>
                          </span>
                        </div>
                      </td>
                      <td>
                        <span
                          className={
                            "pill " + (u.role === "admin" ? "info" : "neutral")
                          }
                        >
                          {roleLabel({ ...u, builtin: false })}
                        </span>
                      </td>
                      <td className="wrap">
                        {u.role === "admin" ? (
                          <span className="muted">{t("users.fullAccess")}</span>
                        ) : (
                          <GrantList
                            grants={u.permissions}
                            connectionName={connectionName}
                          />
                        )}
                      </td>
                      <td>
                        <span
                          className={
                            "pill " + (u.disabled ? "danger" : "success")
                          }
                        >
                          <i />
                          {u.disabled ? t("users.locked") : t("common.active")}
                        </span>
                      </td>
                      <td>{date(u.created_at)}</td>
                      <td>
                        <Menu
                          items={[
                            [t("common.edit"), () => openUser(u)],
                            ...(u.id !== me!.user_id
                              ? ([
                                  [
                                    t("common.delete"),
                                    () => openDeleteUser(u),
                                    true,
                                  ],
                                ] as [string, () => void, boolean][])
                              : []),
                          ]}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <Empty
              icon={<Users size={28} />}
              title={t("users.empty")}
              text={t("users.emptyText")}
              action={
                <button className="primary" onClick={() => openUser()}>
                  <UserPlus size={16} /> {t("users.add")}
                </button>
              }
            />
          )}
        </div>
      </>
    );
  }

  function NotificationsPage() {
    const eventNames = (n: Notifier) =>
      n.events.map((e) => t(("notify.event." + e) as Key)).join(", ");
    const target = (n: Notifier) =>
      n.type === "telegram"
        ? "chat " + (n.config.chat_id || "")
        : n.type === "email"
          ? n.config.to || ""
          : t("notify.urlHidden");
    return (
      <>
        {pageHead(
          t("nav.notifications"),
          t("notify.subtitle"),
          <button className="primary" onClick={() => openNotifier()}>
            <Plus size={16} /> {t("notify.add")}
          </button>,
        )}
        {notifiers.length ? (
          <div className="dest-list">
            {notifiers.map((n) => (
              <div
                className={"card dest-card" + (n.enabled ? "" : " off")}
                key={n.id}
              >
                <header>
                  <span className="dest-icon">
                    <Bell size={18} />
                  </span>
                  <div className="dest-title">
                    <strong title={n.name}>{n.name}</strong>
                    <small>
                      {notifierTypeName(n.type)} · {target(n)}
                    </small>
                  </div>
                  <Menu
                    items={[
                      [t("common.edit"), () => openNotifier(n)],
                      [
                        n.enabled ? t("notify.pause") : t("notify.resume"),
                        () => toggleNotifier(n),
                      ],
                      [t("common.delete"), () => openDeleteNotifier(n), true],
                    ]}
                  />
                </header>
                <dl className="kv">
                  <dt>{t("notify.events")}</dt>
                  <dd>{eventNames(n)}</dd>
                  <dt>{t("notifyForm.lang")}</dt>
                  <dd>{n.lang.toUpperCase()}</dd>
                  <dt>{t("common.status")}</dt>
                  <dd>
                    <span
                      className={"pill " + (n.enabled ? "success" : "neutral")}
                    >
                      <i />
                      {n.enabled ? t("notify.on") : t("notify.off")}
                    </span>
                  </dd>
                </dl>
              </div>
            ))}
          </div>
        ) : (
          <div className="card">
            <Empty
              icon={<Bell size={28} />}
              title={t("notify.empty")}
              text={t("notify.emptyText")}
              action={
                <button className="primary" onClick={() => openNotifier()}>
                  <Plus size={16} /> {t("notify.add")}
                </button>
              }
            />
          </div>
        )}
        <div className="notice">
          <ShieldCheck size={16} />
          <span>{tx("notify.note")}</span>
        </div>
      </>
    );
  }

  // --------------------------------------------------------------- layout --

  const accountNav: [string, string, typeof Database, number?][] = [
    ["/", t("nav.connections"), Server, profiles.length],
    ["/backups", t("nav.allBackups"), Archive, backups.length],
    ["/schedules", t("nav.schedules"), CalendarClock],
    ["/cloud", t("nav.cloud"), Cloud, destinations.length],
    ["/activity", t("nav.activityLog"), Activity],
    ...(admin
      ? ([
          ["/notifications", t("nav.notifications"), Bell, notifiers.length],
          ["/users", t("nav.users"), Users, users.length],
        ] as [string, string, typeof Database, number?][])
      : []),
  ];
  const isAccountActive = (path: string) =>
    (path === "/" && route.view === "home") || path === "/" + route.view;

  const sidebar =
    route.view === "zone" ? (
      <>
        <a className="back-link" href="#/">
          <ArrowLeft size={15} /> {t("common.allConnections")}
        </a>
        <div className="switcher">
          <button
            className="switcher-btn"
            aria-expanded={switcherOpen}
            onClick={() => setSwitcherOpen(!switcherOpen)}
          >
            {zone ? (
              <>
                <ConnAvatar name={zone.name} small />
                <span>
                  <strong>{zone.name}</strong>
                  <small>{zone.host}</small>
                </span>
              </>
            ) : (
              <span>
                <strong>{t("common.connection")}</strong>
              </span>
            )}
            <ChevronDown size={16} />
          </button>
          {switcherOpen && (
            <div className="switcher-list">
              {profiles.map((p) => (
                <a
                  key={p.id || "_env"}
                  href={"#/c/" + connKey(p.id) + "/" + route.tab}
                  className={p.id === zone?.id ? "current" : ""}
                  onClick={() => setSwitcherOpen(false)}
                >
                  <span
                    className={
                      "dot " +
                      (probes[p.id]?.state === "ok"
                        ? "ok"
                        : probes[p.id]?.state === "error"
                          ? "err"
                          : "")
                    }
                  />
                  <span>
                    {p.name}
                    <small>{p.host}</small>
                  </span>
                  {p.id === zone?.id && <Check size={15} />}
                </a>
              ))}
              {admin && (
                <button
                  onClick={() => {
                    setSwitcherOpen(false);
                    openConnection();
                  }}
                >
                  <Plus size={15} /> {t("conn.add")}
                </button>
              )}
            </div>
          )}
        </div>
        <nav>
          {zoneNav.map(([tab, text, Icon]) => (
            <a
              key={tab}
              href={
                "#/c/" +
                connKey(route.id) +
                (tab === "overview" ? "" : "/" + tab)
              }
              className={"nav-item" + (route.tab === tab ? " active" : "")}
            >
              <Icon size={17} />
              <span>{t(text)}</span>
            </a>
          ))}
        </nav>
      </>
    ) : (
      <>
        <div className="nav-label">{t("nav.account")}</div>
        <nav>
          {accountNav.map(([path, text, Icon, count]) => (
            <a
              key={path}
              href={"#" + path}
              className={"nav-item" + (isAccountActive(path) ? " active" : "")}
            >
              <Icon size={17} />
              <span>{text}</span>
              {count !== undefined && <b>{count}</b>}
            </a>
          ))}
        </nav>
        <div className="nav-sep" />
        <nav>
          <a
            href="#/help"
            className={"nav-item" + (route.view === "help" ? " active" : "")}
          >
            <CircleHelp size={17} />
            <span>{t("nav.help")}</span>
          </a>
        </nav>
      </>
    );

  return (
    <div className="app">
      <header className="topbar">
        <button
          className="icon-btn nav-toggle"
          aria-label={t("common.menu")}
          onClick={() => setNavOpen(!navOpen)}
        >
          <MenuIcon size={20} />
        </button>
        <a className="brand" href="#/">
          <BrandLogo branding={branding} />
          <span className="brand-text">{branding.name}</span>
        </a>
        <nav className="crumbs" aria-label="Breadcrumb">
          <ChevronRight size={14} />
          <a href="#/">
            <Home size={14} /> {t("nav.account")}
          </a>
          {zone && (
            <>
              <ChevronRight size={14} />
              <a href={"#/c/" + connKey(zone.id)}>{zone.name}</a>
            </>
          )}
        </nav>
        <div className="topbar-right">
          {active.length > 0 && (
            <a
              className="pill info job-indicator"
              href={
                zone ? "#/c/" + connKey(zone.id) + "/activity" : "#/activity"
              }
            >
              <LoaderCircle size={12} className="spin" />{" "}
              {t("topbar.jobsRunning", { n: active.length })}
            </a>
          )}
          <a
            className="icon-btn"
            href="#/help"
            title={t("nav.help")}
            aria-label={t("nav.help")}
          >
            <CircleHelp size={18} />
          </a>
          <UserMenu
            me={me}
            openProfile={() => openModal("profile")}
            openUsers={admin ? () => go("/users") : undefined}
            openTimezone={admin ? openTimezone : undefined}
            openBranding={admin ? () => openModal("branding") : undefined}
            logout={() => void logout()}
          />
        </div>
      </header>
      <aside className={"sidebar" + (navOpen ? " open" : "")}>{sidebar}</aside>
      {navOpen && <div className="scrim" onClick={() => setNavOpen(false)} />}
      <main className="content">
        <div className="content-inner">
          {error && !modal && (
            <div className="alert">
              <AlertCircle size={16} />
              <div>{error}</div>
              <button className="icon-btn" onClick={() => setError("")}>
                <X size={15} />
              </button>
            </div>
          )}
          {route.view === "home" && HomePage()}
          {route.view === "zone" && ZonePage()}
          {route.view === "backups" && (
            <>
              {pageHead(
                t("nav.allBackups"),
                t("backups.subtitle"),
                operable.length > 0 && (
                  <>
                    <button className="secondary" onClick={() => openUpload()}>
                      <ArrowUpFromLine size={16} /> {t("action.upload")}
                    </button>
                    <button className="primary" onClick={() => openBackup()}>
                      <Plus size={16} /> {t("action.backupNow")}
                    </button>
                  </>
                ),
              )}
              {BackupsPanel({})}
            </>
          )}
          {route.view === "cloud" && CloudPage()}
          {route.view === "users" && UsersPage()}
          {route.view === "notifications" && admin && NotificationsPage()}
          {route.view === "help" && <HelpPage section={route.section} />}
          {route.view === "schedules" && (
            <>
              {pageHead(
                t("nav.schedules"),
                t("schedules.subtitle"),
                operable.length > 0 && (
                  <button className="primary" onClick={() => openSchedule()}>
                    <Plus size={16} /> {t("action.addSchedule")}
                  </button>
                ),
              )}
              {SchedulesPanel({})}
            </>
          )}
          {route.view === "activity" && (
            <>
              {pageHead(t("nav.activityLog"), t("activity.subtitle"))}
              <div className="card">
                <div className="card-head">
                  <h2>{t("activity.jobs")}</h2>
                </div>
                {JobsCard({})}
              </div>
              <div className="card">
                <div className="card-head">
                  <h2>{t("activity.audit")}</h2>
                  <span className="muted">
                    {admin ? t("activity.auditAdmin") : t("activity.auditUser")}
                  </span>
                </div>
                {audit.length ? (
                  <AuditTable audit={audit} />
                ) : (
                  <Empty
                    icon={<ShieldCheck size={28} />}
                    title={t("activity.auditEmpty")}
                    text={t("activity.auditEmptyText")}
                  />
                )}
              </div>
            </>
          )}
          <footer>
            <ShieldCheck size={13} /> {branding.name} · Logical backups · Local
            + encrypted cloud storage
          </footer>
        </div>
      </main>
      {notice && (
        <div className="toast" role="status">
          <CheckCircle2 size={17} />
          {notice}
        </div>
      )}
      {modal && renderModal()}
    </div>
  );

  // --------------------------------------------------------------- modals --

  function renderModal() {
    const connProbe = probes[modalConn];
    const modalDbs = connProbe?.state === "ok" ? connProbe.dbs : [];
    const restoreChoices = restoreTargets.filter(
      (p) => !selected || engineOf(p.engine) === engineOf(selected.engine),
    );
    const restoreProfile = restoreChoices.find((p) => p.id === restoreServer);
    // Restore tests of this connection's backups: same engine, restore on.
    const verifyChoices = restoreTargets.filter(
      (p) =>
        engineOf(p.engine) ===
        engineOf(profiles.find((c) => c.id === modalConn)?.engine),
    );
    const modalEngine =
      engines[engineOf(profiles.find((p) => p.id === modalConn)?.engine)];
    // Backup, schedule and upload only offer what the user may operate on.
    const scopedModal =
      modal === "backup" || modal === "schedule" || modal === "upload";
    const modalProfiles = scopedModal ? operable : profiles;
    const allDbsAllowed = canOp(modalConn, "*");
    const chooseConnection = (
      <label>
        {t("common.connection")}
        <select
          value={modalConn}
          onChange={(e) => {
            setModalConn(e.target.value);
            setDatabase(
              modal === "upload" || modal === "cloudFiles" ? database : "*",
            );
            void probe(e.target.value);
          }}
          disabled={!modalProfiles.length}
        >
          {!modalProfiles.length && <option value="">{t("home.empty")}</option>}
          {modalProfiles.map((p) => (
            <option key={p.id || "_env"} value={p.id}>
              {p.name} · {endpoint(p)}
            </option>
          ))}
        </select>
        {connProbe?.state === "error" && (
          <small className="error-text">{connProbe.error}</small>
        )}
      </label>
    );
    const databaseSelect = (
      <label>
        {t("common.database")}
        <select value={database} onChange={(e) => setDatabase(e.target.value)}>
          {allDbsAllowed ? (
            <option value="*">{t("common.allDatabases")}</option>
          ) : (
            <option value="*" disabled>
              {t("modal.chooseDb")}
            </option>
          )}
          {modalDbs
            .filter((d) => canOp(modalConn, d.name))
            .map((d) => (
              <option key={d.name}>{d.name}</option>
            ))}
          {database !== "*" && !modalDbs.some((d) => d.name === database) && (
            <option>{database}</option>
          )}
        </select>
      </label>
    );
    const destinationSelect = (label: string) => (
      <label>
        {label}
        <select
          value={destinationChoice}
          onChange={(e) => setDestinationChoice(e.target.value)}
        >
          <option value="">{t("modal.noUpload")}</option>
          {destinations.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name} · {providerName(d.type)}
            </option>
          ))}
        </select>
        <small>
          {destinations.length ? t("modal.destHint") : t("modal.destHintEmpty")}
        </small>
      </label>
    );
    const titles: Record<ModalKind, string> = {
      connection: editingConnection ? t("conn.edit") : t("conn.add"),
      deleteConnection: t("conn.delete"),
      backup: t("action.createBackup"),
      restore: t("restore.title"),
      schedule: editingSchedule ? t("schedules.edit") : t("action.addSchedule"),
      upload: t("upload.title"),
      delete: t("backup.delete"),
      destination: editingDestination ? t("dest.edit") : t("dest.add"),
      deleteDestination: t("dest.delete"),
      toCloud: t("job.offsite"),
      cloudFiles: t("modal.cloudFiles", {
        name: editingDestination?.name ?? "cloud",
      }),
      profile: t("profile.title"),
      user: editingUser ? t("users.edit") : t("users.add"),
      deleteUser: t("users.delete"),
      timezone: t("tz.title"),
      verify: t("verify.title"),
      branding: t("branding.title"),
      notifier: editingNotifier ? t("notify.edit") : t("notify.add"),
      deleteNotifier: t("notify.delete"),
    };
    return (
      <Modal
        title={titles[modal!]}
        close={() => {
          if (!busy) setModal(null);
        }}
      >
        {error && <div className="alert">{error}</div>}
        {modal === "timezone" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(async () => {
                await api("/settings/timezone", "PUT", { timezone: tzInput });
                setMe(await api<Me>("/me"));
              }, t("notice.tzChanged"));
            }}
          >
            <label>
              {t("tz.label")}
              <input
                list="tz-options"
                value={tzInput}
                onChange={(e) => setTzInput(e.target.value)}
                placeholder="Asia/Ho_Chi_Minh"
                required
              />
              <datalist id="tz-options">
                {timezones.map((z) => (
                  <option key={z} value={z} />
                ))}
              </datalist>
              <small>{tx("tz.default", { tz: me!.default_timezone })}</small>
            </label>
            <div className="form-note">{t("tz.note")}</div>
            <div className="modal-actions">
              {tzInput !== me!.default_timezone && (
                <button
                  type="button"
                  className="secondary"
                  disabled={busy}
                  onClick={() => setTzInput(me!.default_timezone)}
                >
                  {t("tz.reset")}
                </button>
              )}
              <button
                type="button"
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button className="primary" disabled={busy || !tzInput.trim()}>
                <Globe size={16} /> {t("tz.save")}
              </button>
            </div>
          </form>
        )}
        {modal === "profile" && (
          <>
            <div className="profile-head">
              <span className="avatar lg">
                {initials(me!.display_name || me!.username)}
              </span>
              <div>
                <strong>{me!.display_name || me!.username}</strong>
                <small>@{me!.username}</small>
              </div>
              <span className={"pill " + (admin ? "info" : "neutral")}>
                {roleLabel(me!)}
              </span>
            </div>
            <dl className="kv">
              <dt>{t("menu.timezone")}</dt>
              <dd>
                {me!.timezone}
                {admin && (
                  <>
                    {" "}
                    <button
                      type="button"
                      className="link"
                      onClick={openTimezone}
                    >
                      {t("common.change")}
                    </button>
                  </>
                )}
              </dd>
              <dt>{t("users.permissions")}</dt>
              <dd>
                {admin ? (
                  t("users.fullAccess")
                ) : (
                  <GrantList
                    grants={me!.permissions}
                    connectionName={connectionName}
                  />
                )}
              </dd>
            </dl>
            <h3 className="form-section">{t("profile.changePassword")}</h3>
            {me!.user_id === "env:viewer" ? (
              <p className="form-note">{tx("profile.viewerNote")}</p>
            ) : (
              <>
                <PasswordForm
                  setBusy={setBusy}
                  saved={() => {
                    setModal(null);
                    setNotice(t("notice.passwordChanged"));
                  }}
                />
              </>
            )}
          </>
        )}
        {modal === "user" && (
          <UserForm
            user={editingUser}
            selfId={me!.user_id}
            profiles={profiles}
            probes={probes}
            setBusy={setBusy}
            cancel={() => setModal(null)}
            saved={async () => {
              setModal(null);
              setNotice(
                editingUser ? t("notice.userSaved") : t("notice.userCreated"),
              );
              await refresh();
            }}
          />
        )}
        {modal === "deleteUser" && editingUser && (
          <>
            <p className="modal-intro">
              {tx("users.deleteConfirm", { name: editingUser.username })}
            </p>
            <div className="modal-actions">
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="danger-btn"
                disabled={busy}
                onClick={() =>
                  void action(
                    () => api("/users/" + editingUser.id, "DELETE"),
                    t("notice.userDeleted"),
                  )
                }
              >
                <Trash2 size={15} /> {t("users.delete")}
              </button>
            </div>
          </>
        )}
        {modal === "connection" && (
          <ConnectionForm
            profile={editingConnection}
            saved={async (c) => {
              setModal(null);
              setNotice(t("notice.connSaved"));
              await refresh();
              void probe(c.id, true);
              if (!editingConnection) go("/c/" + connKey(c.id));
            }}
            setBusy={setBusy}
          />
        )}
        {modal === "deleteConnection" && deletingConnection && (
          <>
            <p className="modal-intro">
              {tx("conn.deleteConfirm", { name: deletingConnection.name })}
            </p>
            <div className="modal-actions">
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="danger-btn"
                disabled={busy}
                onClick={() =>
                  void action(async () => {
                    await api(
                      "/connections/" + deletingConnection.id,
                      "DELETE",
                    );
                    if (zone?.id === deletingConnection.id) go("/");
                  }, t("notice.connDeleted"))
                }
              >
                {t("conn.delete")}
              </button>
            </div>
          </>
        )}
        {modal === "backup" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(
                () =>
                  api("/backups", "POST", {
                    database,
                    connection_id: modalConn,
                    destination_id: destinationChoice,
                  }),
                t("notice.backupQueued"),
              );
            }}
          >
            <p className="modal-intro">{t("backup.intro")}</p>
            {chooseConnection}
            {databaseSelect}
            {destinationSelect(t("job.offsite"))}
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  connProbe?.state !== "ok" ||
                  (database === "*" && !allDbsAllowed)
                }
              >
                {busy ? (
                  <LoaderCircle className="spin" size={16} />
                ) : (
                  <Plus size={16} />
                )}
                {t("backup.start")}
              </button>
            </div>
          </form>
        )}
        {modal === "restore" && selected && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(
                () =>
                  api("/restore", "POST", {
                    backup_id: selected.id,
                    connection_id: restoreServer,
                    target,
                    confirmation,
                    trusted,
                  }),
                t("notice.restoreQueued"),
              );
            }}
          >
            <div className="notice">
              <ShieldCheck size={16} />
              <span>{tx("restore.notice")}</span>
            </div>
            <div className="selected-backup">
              <Database size={18} />
              <div>
                <strong>{selected.database}</strong>
                <small>
                  {connectionName(selected.connection_id)} ·{" "}
                  {date(selected.created_at)} · {size(selected.size)}
                </small>
              </div>
            </div>
            <label>
              {t("restore.target")}
              <select
                value={restoreServer}
                onChange={(e) => setRestoreServer(e.target.value)}
                required
              >
                {!restoreChoices.length && (
                  <option value="">
                    {t("restore.noTarget", {
                      engine: engines[engineOf(selected.engine)].label,
                    })}
                  </option>
                )}
                {restoreChoices.map((p) => (
                  <option key={p.id || "_env"} value={p.id}>
                    {p.name} · {endpoint(p)}
                  </option>
                ))}
              </select>
              <small>
                {t(
                  selected.engine === "files"
                    ? "restore.folder"
                    : "restore.account",
                  {
                    user: restoreProfile?.restore_username || "—",
                    dir: restoreProfile?.restore_username || "—",
                  },
                )}
              </small>
            </label>
            <label>
              {selected.engine === "files"
                ? t("restore.targetFolder")
                : t("restore.targetName")}
              <input
                pattern="[a-zA-Z_][a-zA-Z0-9_-]{0,62}"
                value={target}
                onChange={(e) => setTarget(e.target.value)}
                required
              />
            </label>
            <label>
              {t("restore.confirmName")}
              <input
                value={confirmation}
                onChange={(e) => setConfirmation(e.target.value)}
                placeholder={target}
                required
              />
            </label>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={trusted}
                onChange={(e) => setTrusted(e.target.checked)}
              />
              {t("restore.trust")}
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  !restoreProfile?.restore_enabled ||
                  !trusted ||
                  !target ||
                  target !== confirmation
                }
              >
                {busy ? (
                  <LoaderCircle size={16} className="spin" />
                ) : (
                  <RefreshCw size={16} />
                )}
                {t("restore.submit")}
              </button>
            </div>
          </form>
        )}
        {modal === "schedule" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(
                () =>
                  api("/schedules", "POST", {
                    ...(editingSchedule ? { id: editingSchedule.id } : {}),
                    database,
                    connection_id: modalConn,
                    cron,
                    retention_days: days,
                    enabled: editingSchedule ? editingSchedule.enabled : true,
                    destination_id: destinationChoice,
                    verify: scheduleVerify,
                    verify_connection_id: scheduleVerify
                      ? scheduleVerifyConn
                      : "",
                  }),
                editingSchedule
                  ? t("notice.scheduleUpdated")
                  : t("notice.scheduleCreated"),
              );
            }}
          >
            {chooseConnection}
            {databaseSelect}
            <div className="preset-row">
              {(
                [
                  ["0 2 * * *", t("schedule.daily")],
                  ["0 */6 * * *", t("schedule.every6h")],
                  ["0 2 * * 0", t("schedule.sunday")],
                ] as const
              ).map(([c, text]) => (
                <button
                  key={c}
                  type="button"
                  className={"chip" + (cron === c ? " on" : "")}
                  onClick={() => setCron(c)}
                >
                  {text}
                </button>
              ))}
            </div>
            <div className="form-row">
              <label>
                {t("schedule.cron")}
                <input
                  value={cron}
                  onChange={(e) => setCron(e.target.value)}
                  required
                />
                <small>{t("schedule.cronHint", { tz: me!.timezone })}</small>
              </label>
              <label className="narrow">
                {t("schedule.retention")}
                <input
                  type="number"
                  min="1"
                  max="3650"
                  value={days}
                  onChange={(e) => setDays(Number(e.target.value))}
                  required
                />
              </label>
            </div>
            {destinationSelect(t("schedule.sendCloud"))}
            <label className="checkbox">
              <input
                type="checkbox"
                checked={scheduleVerify}
                disabled={!verifyChoices.length}
                onChange={(e) => {
                  setScheduleVerify(e.target.checked);
                  // Default to the schedule's own server when it can restore.
                  setScheduleVerifyConn(
                    verifyChoices.some((p) => p.id === modalConn)
                      ? ""
                      : (verifyChoices[0]?.id ?? ""),
                  );
                }}
              />
              <span>
                {t("schedule.verify")}
                <br />
                <small className="muted">
                  {verifyChoices.length
                    ? t("schedule.verifyHint")
                    : t("schedule.verifyUnavailable")}
                </small>
              </span>
            </label>
            {scheduleVerify && (
              <label>
                {t("schedule.verifyServer")}
                <select
                  value={scheduleVerifyConn}
                  onChange={(e) => setScheduleVerifyConn(e.target.value)}
                >
                  {verifyChoices.map((p) => (
                    <option
                      key={p.id || "_env"}
                      value={p.id === modalConn ? "" : p.id}
                    >
                      {p.name} · {endpoint(p)}
                      {p.id === modalConn
                        ? " (" + t("schedule.sameServer") + ")"
                        : ""}
                    </option>
                  ))}
                </select>
                <small>{t("schedule.verifyServerHint")}</small>
              </label>
            )}
            <div className="form-note">{t("schedule.note")}</div>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="primary"
                disabled={
                  busy ||
                  !modalProfiles.length ||
                  (database === "*" && !allDbsAllowed)
                }
              >
                <CalendarClock size={16} /> {t("schedule.save")}
              </button>
            </div>
          </form>
        )}
        {modal === "upload" && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(async () => {
                if (!file) throw new Error(t("upload.chooseFile"));
                const res = await fetch("/api/upload", {
                  method: "POST",
                  headers: {
                    "X-Requested-With": "DBBackupManager",
                    "X-Database-Name": database,
                    "X-Connection-ID": modalConn,
                    "Content-Type": "application/octet-stream",
                  },
                  body: file,
                });
                const data = await res.json();
                if (!res.ok) throw new Error(translateServer(data.error));
              }, t("notice.uploaded"));
            }}
          >
            <p className="modal-intro">
              {t("upload.intro", {
                size: size(me!.max_upload_bytes),
                format: modalEngine.format,
              })}
            </p>
            {chooseConnection}
            <label>
              {t("upload.dbName")}
              <input
                value={database}
                onChange={(e) => setDatabase(e.target.value)}
                pattern="[a-zA-Z_][a-zA-Z0-9_-]{0,62}"
                required
              />
            </label>
            <label className="file-drop">
              <ArrowUpFromLine size={22} />
              <span>
                {file
                  ? file.name
                  : t("upload.pick", { ext: modalEngine.accept })}
              </span>
              <input
                type="file"
                accept={modalEngine.accept}
                onChange={(e) => setFile(e.target.files?.[0] || null)}
                required
              />
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button className="primary" disabled={busy || !file}>
                {busy ? (
                  <LoaderCircle size={16} className="spin" />
                ) : (
                  <ArrowUpFromLine size={16} />
                )}
                {t("upload.submit")}
              </button>
            </div>
          </form>
        )}
        {modal === "branding" && (
          <BrandingForm
            branding={branding}
            cancel={() => setModal(null)}
            saved={async () => {
              setModal(null);
              setNotice(t("notice.brandingSaved"));
              await loadBranding();
            }}
            setBusy={setBusy}
          />
        )}
        {modal === "verify" && selected && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(
                () =>
                  api("/backups/" + selected.id + "/verify", "POST", {
                    connection_id: verifyServer,
                  }),
                t("notice.verifyQueued"),
              );
            }}
          >
            <p className="modal-intro">{tx("verify.intro")}</p>
            <div className="selected-backup">
              <Database size={18} />
              <div>
                <strong>{selected.database}</strong>
                <small>
                  {connectionName(selected.connection_id)} ·{" "}
                  {date(selected.created_at)} · {size(selected.size)}
                </small>
              </div>
            </div>
            <label>
              {t("verify.server")}
              <select
                value={verifyServer}
                onChange={(e) => setVerifyServer(e.target.value)}
                required
              >
                {!restoreChoices.length && (
                  <option value="">
                    {t("restore.noTarget", {
                      engine: engines[engineOf(selected.engine)].label,
                    })}
                  </option>
                )}
                {restoreChoices.map((p) => (
                  <option key={p.id || "_env"} value={p.id}>
                    {p.name} · {endpoint(p)}
                  </option>
                ))}
              </select>
              <small>{t("verify.serverHint")}</small>
            </label>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="primary"
                disabled={busy || !restoreChoices.length}
              >
                {busy ? (
                  <LoaderCircle size={16} className="spin" />
                ) : (
                  <FlaskConical size={16} />
                )}
                {t("verify.submit")}
              </button>
            </div>
          </form>
        )}
        {modal === "notifier" && (
          <NotifierForm
            notifier={editingNotifier}
            saved={async () => {
              setModal(null);
              setNotice(t("notice.notifierSaved"));
              await refresh();
            }}
            setBusy={setBusy}
          />
        )}
        {modal === "deleteNotifier" && editingNotifier && (
          <>
            <p className="modal-intro">
              {tx("notify.deleteConfirm", { name: editingNotifier.name })}
            </p>
            <div className="modal-actions">
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="danger-btn"
                disabled={busy}
                onClick={() =>
                  void action(
                    () => api("/notifiers/" + editingNotifier.id, "DELETE"),
                    t("notice.notifierDeleted"),
                  )
                }
              >
                <Trash2 size={15} /> {t("notify.delete")}
              </button>
            </div>
          </>
        )}
        {modal === "destination" && (
          <DestinationForm
            destination={editingDestination}
            saved={async () => {
              setModal(null);
              setNotice(t("notice.destSaved"));
              await refresh();
            }}
            setBusy={setBusy}
          />
        )}
        {modal === "deleteDestination" && editingDestination && (
          <>
            <p className="modal-intro">
              {tx("dest.deleteConfirm", { name: editingDestination.name })}
            </p>
            <div className="modal-actions">
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="danger-btn"
                disabled={busy}
                onClick={() =>
                  void action(
                    () =>
                      api("/destinations/" + editingDestination.id, "DELETE"),
                    t("notice.destDeleted"),
                  )
                }
              >
                <Trash2 size={15} /> {t("dest.delete")}
              </button>
            </div>
          </>
        )}
        {modal === "toCloud" && selected && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void action(
                () =>
                  api("/backups/" + selected.id + "/offsite", "POST", {
                    destination_id: destinationChoice,
                  }),
                t("notice.offsiteQueued"),
              );
            }}
          >
            <div className="selected-backup">
              <Database size={18} />
              <div>
                <strong>{selected.database}</strong>
                <small>
                  {connectionName(selected.connection_id)} ·{" "}
                  {date(selected.created_at)} · {size(selected.size)}
                </small>
              </div>
            </div>
            <label>
              {t("toCloud.dest")}
              <select
                value={destinationChoice}
                onChange={(e) => setDestinationChoice(e.target.value)}
                required
              >
                {destinations.map((d) => (
                  <option key={d.id} value={d.id}>
                    {d.name} · {providerName(d.type)}
                    {offsite.some(
                      (o) =>
                        o.backup_id === selected.id &&
                        o.destination_id === d.id,
                    )
                      ? " " + t("toCloud.exists")
                      : ""}
                  </option>
                ))}
              </select>
            </label>
            <div className="form-note">
              <ShieldCheck size={16} />
              {t("toCloud.note")}
            </div>
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button className="primary" disabled={busy || !destinationChoice}>
                <CloudUpload size={16} /> {t("toCloud.submit")}
              </button>
            </div>
          </form>
        )}
        {modal === "cloudFiles" && editingDestination && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (!remotePick) return;
              void action(
                () =>
                  api(
                    "/destinations/" + editingDestination.id + "/pull",
                    "POST",
                    {
                      remote_path: remotePick.path,
                      database,
                      connection_id: modalConn,
                      secret: pullSecret,
                    },
                  ),
                t("notice.pullQueued"),
              );
            }}
          >
            <p className="modal-intro">
              {t("pull.intro", { source: t("source.cloud") })}
            </p>
            {remote.state === "loading" && (
              <div className="loading inline">
                <LoaderCircle className="spin" /> {t("pull.loading")}
              </div>
            )}
            {remote.state === "error" && (
              <div className="alert">{remote.error}</div>
            )}
            {remote.state === "ok" &&
              (remote.files.length ? (
                <div className="remote-list" role="listbox">
                  {remote.files.map((f) => (
                    <button
                      type="button"
                      key={f.path}
                      role="option"
                      aria-selected={remotePick?.path === f.path}
                      className={remotePick?.path === f.path ? "on" : ""}
                      onClick={() => {
                        setRemotePick(f);
                        setDatabase(f.database);
                      }}
                    >
                      <Cloud size={16} />
                      <span>
                        <code title={f.path}>{f.path}</code>
                        <small>
                          {date(f.mod_time)} · {size(f.size)}
                          {f.backup_id && " · " + t("pull.onServer")}
                        </small>
                      </span>
                      {remotePick?.path === f.path && <Check size={16} />}
                    </button>
                  ))}
                </div>
              ) : (
                <Empty
                  icon={<Cloud size={28} />}
                  title={t("pull.empty")}
                  text={t("pull.emptyText", {
                    path: editingDestination.remote_path,
                  })}
                />
              ))}
            {admin && remotePick && (
              <>
                <div className="form-row">
                  {chooseConnection}
                  <label>
                    {t("pull.dbName")}
                    <input
                      value={database}
                      onChange={(e) => setDatabase(e.target.value)}
                      pattern="[a-zA-Z_][a-zA-Z0-9_-]{0,62}"
                      required
                    />
                  </label>
                </div>
                <label>
                  {editingDestination.encryption === "recipient"
                    ? t("pull.privateKey")
                    : t("pull.otherPassphrase")}
                  <textarea
                    className="mono"
                    rows={2}
                    spellCheck={false}
                    autoComplete="off"
                    value={pullSecret}
                    onChange={(e) => setPullSecret(e.target.value)}
                    placeholder={
                      editingDestination.encryption === "recipient"
                        ? "AGE-SECRET-KEY-1…"
                        : t("pull.passphrasePlaceholder")
                    }
                    required={editingDestination.encryption === "recipient"}
                  />
                  <small>{t("pull.secretHint")}</small>
                </label>
              </>
            )}
            <div className="modal-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => setModal(null)}
              >
                {t("common.close")}
              </button>
              {admin && (
                <button className="primary" disabled={busy || !remotePick}>
                  <CloudDownload size={16} /> {t("pull.submit")}
                </button>
              )}
            </div>
          </form>
        )}
        {modal === "delete" && selected && (
          <>
            <p className="modal-intro">
              {tx("backup.deleteConfirm", {
                db: selected.database,
                date: date(selected.created_at),
                size: size(selected.size),
              })}
            </p>
            <div className="checksum">
              SHA-256
              <br />
              {selected.checksum}
            </div>
            <div className="modal-actions">
              <button
                className="secondary"
                disabled={busy}
                onClick={() => setModal(null)}
              >
                {t("common.cancel")}
              </button>
              <button
                className="danger-btn"
                disabled={busy}
                onClick={() =>
                  action(
                    () => api("/backups/" + selected.id, "DELETE"),
                    t("notice.backupDeleted"),
                  )
                }
              >
                <Trash2 size={15} /> {t("backup.delete")}
              </button>
            </div>
          </>
        )}
      </Modal>
    );
  }
}
