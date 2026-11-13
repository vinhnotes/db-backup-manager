import { Fragment, ReactNode, useEffect } from "react";
import {
  Archive,
  Bell,
  CalendarClock,
  Cloud,
  LifeBuoy,
  RotateCcw,
  Rocket,
  Server,
  Users,
} from "lucide-react";
import { getLang, Key, Lang, t } from "./i18n";

/**
 * In-app user guide. Text supports **bold** and `code`; blocks are
 * paragraphs (string), bullet lists, numbered steps, code and notes.
 */
type Block =
  | string
  | { list: string[] }
  | { steps: string[] }
  | { code: string }
  | { note: string };

type Section = { id: string; title: string; blocks: Block[] };

const icons: Record<string, typeof Server> = {
  start: Rocket,
  connections: Server,
  backups: Archive,
  restore: RotateCcw,
  schedules: CalendarClock,
  cloud: Cloud,
  users: Users,
  notifications: Bell,
  troubleshooting: LifeBuoy,
};

const cronExamples = `0 2 * * *      # every day at 02:00
30 1 * * 1-5   # 01:30, Monday to Friday
0 */6 * * *    # every 6 hours
0 3 1 * *      # 03:00 on the 1st of every month`;

const resetAdmin = `docker compose exec db-backup-manager db-backup-manager reset-admin-password`;

const ageDecrypt = `rclone copy remote:db-backups/<connection>/<db>/<file>.dump.age .
age -d -i key.txt <file>.dump.age > <file>.dump`;

const content: Record<Lang, Section[]> = {
  vi: [
    {
      id: "start",
      title: "Bắt đầu nhanh",
      blocks: [
        "Các bước để có bản backup đầu tiên và chạy tự động hằng ngày:",
        {
          steps: [
            "Vào **[[nav.connections]] → [[conn.add]]**, chọn loại database, nhập host, port, tài khoản backup. Bấm **[[settings.check]]** rồi lưu lại.",
            "Mở kết nối vừa tạo, tab **[[nav.databases]]** hiển thị danh sách DB và dung lượng.",
            "Bấm **[[action.backupNow]]**, chọn một database hoặc *[[common.allDatabases]]*.",
            "Theo dõi job trong tab **[[nav.activity]]**. Chỉ job *[[status.completed]]* mới tạo ra bản backup để tải về/restore.",
            "Vào **[[nav.schedules]] → [[action.addSchedule]]**, ví dụ `0 2 * * *` (02:00 mỗi ngày), retention 14 ngày.",
            "(Khuyến nghị) Thêm **[[nav.cloud]]** và gắn vào lịch để có bản sao mã hóa ngoài máy chủ.",
          ],
        },
        {
          note: "Định kỳ thử restore một bản backup vào database mới và kiểm tra dữ liệu từ ứng dụng. Backup chưa từng restore thử thì chưa chắc dùng được.",
        },
      ],
    },
    {
      id: "connections",
      title: "Kết nối database",
      blocks: [
        "Hỗ trợ **PostgreSQL**, **MySQL/MariaDB**, **MongoDB**, **SQL Server** và thư mục file (**Files**). Chỉ quản trị viên được thêm/sửa/xóa kết nối.",
        {
          list: [
            "**Host** là địa chỉ nhìn từ máy chủ/container chạy manager. `localhost` trong container **không** trỏ về máy của bạn.",
            "**Tài khoản backup**: chỉ cần quyền đọc (ví dụ `pg_read_all_data` trên PostgreSQL, role `backup` trên MongoDB). Không nên dùng superuser.",
            "**[[connForm.restoreUser]]** (tùy chọn): tài khoản riêng có quyền tạo database. Để trống để tắt restore cho kết nối này.",
            "**[[settings.check]]** xác minh đăng nhập backup và quyền tạo DB của tài khoản restore.",
            "Mật khẩu được mã hóa trên máy chủ và không bao giờ trả về trình duyệt. Khi sửa, để trống mật khẩu để giữ giá trị cũ.",
            "**Files** backup một thư mục đã mount vào container (ví dụ thư mục upload) thành `.tar.gz`. Khai báo **[[connForm.sourceDir]]** và, nếu cần restore, **[[connForm.restoreDir]]**: restore luôn giải nén vào một thư mục con mới, không ghi đè.",
          ],
        },
        "Không sửa/xóa được kết nối đang có job chạy. Xóa kết nối không xóa các bản backup và lịch sử đã có.",
      ],
    },
    {
      id: "backups",
      title: "Backup, tải về và upload",
      blocks: [
        {
          list: [
            "**[[action.backupNow]]** trên trang kết nối hoặc trang **[[nav.allBackups]]**. Mỗi bản được kiểm tra (định dạng + SHA-256) trước khi lưu.",
            "Định dạng file: PostgreSQL `.dump`, MySQL `.sql.gz`, MongoDB `.archive.gz`, SQL Server `.bak`, Files `.tar.gz`.",
            "Lọc danh sách theo kết nối và database; số lượng và dung lượng cập nhật theo bộ lọc.",
            "**[[action.upload]]** để nhập một bản backup có sẵn (mặc định tối đa 10 GiB). File phải đúng định dạng của loại database được chọn.",
            "Backup thủ công và upload được giữ đến khi xóa tay; retention chỉ dọn bản tạo theo lịch.",
            "Job chạy lần lượt từng cái; trang tự cập nhật mỗi 5 giây.",
          ],
        },
      ],
    },
    {
      id: "restore",
      title: "Khôi phục (restore)",
      blocks: [
        "Restore luôn tạo **database mới**, không bao giờ ghi đè database đang có.",
        {
          steps: [
            "Bấm biểu tượng restore trên một bản backup.",
            "Chọn kết nối đích (cùng loại database, đã cấu hình tài khoản restore). Có thể khác kết nối nguồn.",
            "Nhập tên database mới, nhập lại để xác nhận, và xác nhận bạn tin cậy nguồn của file backup.",
            "Theo dõi job trong **[[nav.activity]]**. Sau khi xong, kiểm tra dữ liệu rồi mới cấp quyền cho ứng dụng.",
          ],
        },
        {
          note: "Restore lỗi giữa chừng sẽ giữ lại database đích để kiểm tra. Muốn thử lại hãy dùng tên khác hoặc tự xóa DB lỗi.",
        },
      ],
    },
    {
      id: "schedules",
      title: "Lịch chạy và retention",
      blocks: [
        "Lịch dùng biểu thức cron 5 trường: `phút giờ ngày tháng thứ`.",
        { code: cronExamples },
        {
          list: [
            "Lịch chạy theo múi giờ của hệ thống. Quản trị viên đổi tại menu tài khoản (góc trên phải) → **[[menu.changeTimezone]]**.",
            "**Retention** (1–3650 ngày) chỉ xóa bản backup tạo theo lịch, chạy sau mỗi lượt backup theo lịch và lúc 03:17 hằng ngày.",
            "Nhiều lịch cùng khớp một database thì dùng retention dài nhất. Tắt/xóa lịch cũng dừng retention cho DB đó.",
            "Lịch bị lỡ khi ứng dụng dừng sẽ **chạy bù một lần** lúc khởi động (lỡ nhiều lượt vẫn chỉ một bản backup).",
            "Backup lỗi được **tự thử lại** (mặc định 2 lần) trước khi báo thất bại.",
            "Có thể gắn đích cloud vào lịch để tự động đẩy bản backup lên cloud.",
          ],
        },
      ],
    },
    {
      id: "cloud",
      title: "Lưu trữ cloud và mã hóa",
      blocks: [
        "Hỗ trợ Google Drive, OneDrive, Dropbox, S3/R2/MinIO, B2, SFTP, WebDAV… qua rclone. File được **mã hóa trên máy chủ** trước khi upload; cloud chỉ nhận file `.age`.",
        {
          steps: [
            "Google Drive: chọn **[[googleOAuth.option]]**, bấm **[[googleOAuth.connect]]**, đăng nhập Google rồi quay lại lưu đích cloud. Quản trị viên cấu hình OAuth Client trên server theo README; dùng localhost qua SSH tunnel, không cần public server.",
            "Cách thủ công / cloud khác: chạy `rclone config`, rồi dán `rclone config show <tên-remote>` vào ô **[[destForm.config]]**. Với S3 có thể nhập cấu hình trực tiếp.",
            "Chọn mã hóa: **[[destForm.publicKeyRec]]** – bấm **[[destForm.genKey]]** và lưu private key; hoặc **Passphrase**.",
            "Bấm **[[destForm.test]]** để thử ghi/đọc/xóa một file trên cloud.",
          ],
        },
        {
          note: "Private key chỉ hiển thị **một lần** và máy chủ không lưu. Cất nó tách khỏi máy chủ và tài khoản cloud. Mất khóa = mất backup.",
        },
        "Để khôi phục: **[[nav.cloud]] → [[cloud.browse]]**, chọn file, nhập private key. Bản giải mã xuất hiện trong danh sách backup với nguồn *[[source.cloud]]*, sau đó restore như bình thường. Giải mã thủ công khi mất máy chủ:",
        { code: ageDecrypt },
      ],
    },
    {
      id: "users",
      title: "Người dùng và phân quyền",
      blocks: [
        {
          list: [
            "**Quản trị viên**: toàn quyền – kết nối, đích cloud, người dùng, mọi database.",
            "**Người dùng**: chỉ thấy phạm vi được cấp. Mỗi quyền gồm *Kết nối* × *Database* (để trống = tất cả) × *Mức*.",
            "Mức **Xem**: xem backup, lịch, job. Mức **Vận hành**: backup, tải về, upload, quản lý lịch, xóa backup, đẩy lên cloud.",
            "Backup/lịch cho *[[common.allDatabases]]* và restore cần quyền vận hành trên **toàn bộ** database của kết nối.",
            "Mỗi người tự đổi mật khẩu trong menu tài khoản → **[[menu.profile]]**.",
          ],
        },
        "Quên mật khẩu admin: chạy lệnh dưới đây trên máy chủ, sau đó đăng nhập bằng `ADMIN_PASSWORD` trong `.env`.",
        { code: resetAdmin },
      ],
    },
    {
      id: "notifications",
      title: "Thông báo và kiểm thử restore",
      blocks: [
        "Quản trị viên vào **[[nav.notifications]] → [[notify.add]]** để nhận tin qua Telegram, Slack, Discord, email hoặc webhook. Bấm **[[notifyForm.test]]** trước khi lưu.",
        {
          list: [
            "**[[notify.event.failure]]**: backup, restore, kiểm thử restore, upload cloud lỗi (backup chỉ báo sau lần thử lại cuối).",
            "**[[notify.event.stale]]**: lịch đã quá giờ 15 phút mà không có job nào, ví dụ container bị dừng.",
            "**[[notify.event.connection]]**: kiểm tra mọi kết nối mỗi 5 phút, báo khi lỗi 2 lần liên tiếp và báo lại khi kết nối khôi phục.",
            "**[[notify.event.overdue]]**: quá `BACKUP_MAX_AGE` (mặc định 24h, hoặc 1 chu kỳ lịch + `JOB_TIMEOUT` nếu dài hơn) chưa có backup thành công.",
            "**[[notify.event.backup_size]]**: bản mới nhỏ hơn bản trước quá `BACKUP_SHRINK_PERCENT` (mặc định 50%).",
            "**[[notify.event.destination]]**: nơi lưu cloud lỗi 2 lần kiểm tra liên tiếp (30 phút/lần) và khi hoạt động lại.",
            "**[[notify.event.disk]]**: ổ chứa thư mục backup còn trống dưới `DISK_ALERT_PERCENT` (mặc định 10%).",
            "**[[notify.event.restore]]**: có người yêu cầu restore.",
            "**[[notify.event.security]]**: một IP đăng nhập sai 10 lần và bị khóa 15 phút.",
            "**[[notify.event.success]]**: job hoàn tất – tùy chọn vì khá nhiều tin.",
          ],
        },
        "**Kiểm thử restore** restore bản backup vào database tạm `dbbm_verify_…`, đếm số bảng rồi xóa database tạm. Chạy tay bằng nút bình thí nghiệm ở mỗi bản backup, hoặc bật *[[schedule.verify]]* khi tạo lịch.",
        {
          note: "Kiểm thử dùng tài nguyên và dung lượng đĩa của Server kiểm thử. Với database lớn, nên chọn Server staging thay vì production.",
        },
      ],
    },
    {
      id: "troubleshooting",
      title: "Xử lý sự cố",
      blocks: [
        {
          list: [
            "**Không kết nối được**: kiểm tra host/port nhìn từ container manager (cùng Docker network), firewall, SSL mode và tài khoản.",
            "**Job thất bại**: mở tab **[[nav.activity]]**, xem cột lỗi của job. Lỗi quyền thường do tài khoản backup/restore thiếu quyền.",
            "**PostgreSQL báo lỗi version**: client trong image (`PG_MAJOR`) phải cùng hoặc mới hơn version server.",
            "**SQL Server không đọc được file .bak**: container phải thuộc group của SQL Server (`group_add`) và thư mục chung không được dùng sticky bit.",
            "**Upload bị từ chối**: kiểm tra định dạng file đúng loại database và dung lượng dưới giới hạn.",
            "**Job đang chạy khi restart** sẽ được đánh dấu thất bại, không tự chạy lại – hãy chạy lại thủ công.",
          ],
        },
      ],
    },
  ],

  en: [
    {
      id: "start",
      title: "Getting started",
      blocks: [
        "Steps to get your first backup and a daily automatic one:",
        {
          steps: [
            "Go to **[[nav.connections]] → [[conn.add]]**, choose the database type, enter host, port and a backup account. Click **[[settings.check]]**, then save.",
            "Open the new connection; the **[[nav.databases]]** tab lists databases and their sizes.",
            "Click **[[action.backupNow]]** and pick one database or *[[common.allDatabases]]*.",
            "Follow the job in the **[[nav.activity]]** tab. Only *[[status.completed]]* jobs produce a backup you can download or restore.",
            "Go to **[[nav.schedules]] → [[action.addSchedule]]**, e.g. `0 2 * * *` (02:00 daily) with 14 days retention.",
            "(Recommended) Add **[[nav.cloud]]** and attach it to the schedule for an encrypted off-site copy.",
          ],
        },
        {
          note: "Regularly test-restore a backup into a new database and check the data from your application. A backup that was never restored is not proven to work.",
        },
      ],
    },
    {
      id: "connections",
      title: "Database connections",
      blocks: [
        "Supports **PostgreSQL**, **MySQL/MariaDB**, **MongoDB**, **SQL Server** and file folders (**Files**). Only admins can add, edit or delete connections.",
        {
          list: [
            "**Host** is resolved from the server/container running the manager. `localhost` inside the container does **not** point to your machine.",
            "**Backup account**: read access is enough (e.g. `pg_read_all_data` on PostgreSQL, the `backup` role on MongoDB). Avoid superusers.",
            "**[[connForm.restoreUser]]** (optional): a separate account allowed to create databases. Leave empty to disable restore for this connection.",
            "**[[settings.check]]** verifies the backup login and the restore account's permission to create databases.",
            "Passwords are encrypted on the server and never sent back to the browser. When editing, leave a password empty to keep it.",
            "**SQL Server** needs a folder shared between SQL Server and the manager container; enter both the SQL Server-side path and the in-container path.",
            "**Files** backs up a folder mounted into the container (e.g. an upload folder) as `.tar.gz`. Enter the **[[connForm.sourceDir]]** and, for restores, a **[[connForm.restoreDir]]**: restores always extract into a new subfolder and never overwrite.",
          ],
        },
        "Connections with running jobs cannot be edited or deleted. Deleting a connection keeps its existing backups and history.",
      ],
    },
    {
      id: "backups",
      title: "Backups, downloads and uploads",
      blocks: [
        {
          list: [
            "Use **[[action.backupNow]]** on a connection or on **[[nav.allBackups]]**. Each backup is verified (format + SHA-256) before it is saved.",
            "File formats: PostgreSQL `.dump`, MySQL `.sql.gz`, MongoDB `.archive.gz`, SQL Server `.bak`, Files `.tar.gz`.",
            "Filter by connection and database; counts and sizes follow the filter.",
            "**[[action.upload]]** imports an existing backup (10 GiB max by default). The file must match the selected database type.",
            "Manual and uploaded backups are kept until deleted; retention only removes scheduled ones.",
            "Jobs run one at a time; the page refreshes every 5 seconds.",
          ],
        },
      ],
    },
    {
      id: "restore",
      title: "Restore",
      blocks: [
        "Restore always creates a **new database**; it never overwrites an existing one.",
        {
          steps: [
            "Click the restore icon on a backup.",
            "Pick a target connection (same database type, with a restore account). It can differ from the source.",
            "Enter the new database name, type it again to confirm, and confirm you trust the backup's source.",
            "Follow the job in **[[nav.activity]]**. Check the data before granting your application access.",
          ],
        },
        {
          note: "If a restore fails midway, the target database is kept for inspection. To retry, use another name or drop the failed database yourself.",
        },
      ],
    },
    {
      id: "schedules",
      title: "Schedules and retention",
      blocks: [
        "Schedules use 5-field cron: `minute hour day month weekday`.",
        { code: cronExamples },
        {
          list: [
            "Schedules follow the system timezone. Admins change it from the account menu (top right) → **[[menu.changeTimezone]]**.",
            "**Retention** (1–3650 days) only deletes scheduled backups. It runs after each scheduled backup and daily at 03:17.",
            "When several schedules match a database, the longest retention wins. Disabling/deleting schedules also stops retention for that database.",
            "A run missed while the app was stopped is **caught up once** at startup (several missed runs still give one backup).",
            "Failed backups are **retried automatically** (twice by default) before they are reported as failed.",
            "Attach a cloud destination to a schedule to upload its backups automatically.",
          ],
        },
      ],
    },
    {
      id: "cloud",
      title: "Cloud storage and encryption",
      blocks: [
        "Supports Google Drive, OneDrive, Dropbox, S3/R2/MinIO, B2, SFTP, WebDAV… via rclone. Files are **encrypted on the server** before upload; the cloud only receives `.age` files.",
        {
          steps: [
            "Google Drive: select **[[googleOAuth.option]]**, click **[[googleOAuth.connect]]**, authorize Google and return to save the destination. Configure server OAuth credentials following README; localhost via an SSH tunnel works without a public server.",
            "Manual setup / other clouds: run `rclone config` and paste `rclone config show <remote-name>` into **[[destForm.config]]**. S3 configuration can be entered directly.",
            "Choose encryption: **[[destForm.publicKeyRec]]** – click **[[destForm.genKey]]** and save the private key; or **Passphrase**.",
            "Click **[[destForm.test]]** to write, read and delete a test file on the cloud.",
          ],
        },
        {
          note: "The private key is shown **only once** and is not stored on the server. Keep it away from the server and the cloud account. Lost key = lost backups.",
        },
        "To recover: **[[nav.cloud]] → [[cloud.browse]]**, pick a file and enter the private key. The decrypted copy appears in the backup list with source *[[source.cloud]]*; restore it as usual. Manual decryption if the server is gone:",
        { code: ageDecrypt },
      ],
    },
    {
      id: "users",
      title: "Users and permissions",
      blocks: [
        {
          list: [
            "**Admin**: full access – connections, cloud destinations, users, every database.",
            "**User**: only sees what was granted. Each grant is *Connection* × *Database* (empty = all) × *Level*.",
            "**View** level: see backups, schedules, jobs. **Operate** level: back up, download, upload, manage schedules, delete backups, send to cloud.",
            "Backups/schedules for *[[common.allDatabases]]* and restores need operate access on **every** database of the connection.",
            "Everyone changes their own password from the account menu → **[[menu.profile]]**.",
          ],
        },
        "Forgot the admin password: run the command below on the server, then sign in with `ADMIN_PASSWORD` from `.env`.",
        { code: resetAdmin },
      ],
    },
    {
      id: "notifications",
      title: "Notifications and restore tests",
      blocks: [
        "Administrators open **[[nav.notifications]] → [[notify.add]]** to get messages on Telegram, Slack, Discord, email or a webhook. Use **[[notifyForm.test]]** before saving.",
        {
          list: [
            "**[[notify.event.failure]]**: a backup, restore, restore test or cloud upload failed (backups only after the last retry).",
            "**[[notify.event.stale]]**: a schedule is 15 minutes past due and created no job, e.g. while the container was down.",
            "**[[notify.event.connection]]**: every connection is checked every 5 minutes; you are told after 2 failed checks in a row and again when it recovers.",
            "**[[notify.event.overdue]]**: no successful backup for longer than `BACKUP_MAX_AGE` (default 24h, or one schedule interval plus `JOB_TIMEOUT` if longer).",
            "**[[notify.event.backup_size]]**: a new backup is more than `BACKUP_SHRINK_PERCENT` (default 50%) smaller than the previous one.",
            "**[[notify.event.destination]]**: a cloud destination failed 2 checks in a row (every 30 minutes), and when it works again.",
            "**[[notify.event.disk]]**: the disk holding the backup folder has less than `DISK_ALERT_PERCENT` (default 10%) free.",
            "**[[notify.event.restore]]**: someone requested a restore.",
            "**[[notify.event.security]]**: an IP address failed to log in 10 times and is locked out for 15 minutes.",
            "**[[notify.event.success]]**: a job completed – optional, as it can be noisy.",
          ],
        },
        "A **restore test** restores the backup into a temporary `dbbm_verify_…` database, counts its tables and drops it. Run it with the flask button on a backup, or turn on *[[schedule.verify]]* when creating a schedule.",
        {
          note: "Tests use resources and disk space on the test server. For large databases, pick a staging server rather than production.",
        },
      ],
    },
    {
      id: "troubleshooting",
      title: "Troubleshooting",
      blocks: [
        {
          list: [
            "**Cannot connect**: check host/port as seen from the manager container (same Docker network), firewall, SSL mode and credentials.",
            "**Job failed**: open the **[[nav.activity]]** tab and read the job's error. Permission errors usually mean the backup/restore account lacks privileges.",
            "**PostgreSQL version error**: the client in the image (`PG_MAJOR`) must be the same as or newer than the server.",
            "**SQL Server cannot read .bak files**: the container must join SQL Server's group (`group_add`) and the shared folder must not use the sticky bit.",
            "**Upload rejected**: check the file format matches the database type and the size is under the limit.",
            "**Jobs running during a restart** are marked failed and not retried – run them again manually.",
          ],
        },
      ],
    },
  ],

  ja: [
    {
      id: "start",
      title: "はじめに",
      blocks: [
        "最初のバックアップと毎日の自動バックアップを設定する手順:",
        {
          steps: [
            "**[[nav.connections]] → [[conn.add]]** でデータベースの種類を選び、ホスト・ポート・バックアップ用アカウントを入力します。**[[settings.check]]** の後、保存します。",
            "作成した接続を開くと、**[[nav.databases]]** タブにデータベースとサイズが表示されます。",
            "**[[action.backupNow]]** をクリックし、データベースまたは *[[common.allDatabases]]* を選びます。",
            "**[[nav.activity]]** タブでジョブを確認します。*[[status.completed]]* したジョブのみがダウンロード・リストア可能なバックアップを作成します。",
            "**[[nav.schedules]] → [[action.addSchedule]]** で、例えば `0 2 * * *`(毎日 02:00)、保持期間 14 日を設定します。",
            "(推奨)**[[nav.cloud]]** を追加してスケジュールに紐付け、暗号化したコピーを外部に保存します。",
          ],
        },
        {
          note: "定期的にバックアップを新しいデータベースへテストリストアし、アプリからデータを確認してください。リストアを試していないバックアップは使えるとは限りません。",
        },
      ],
    },
    {
      id: "connections",
      title: "データベース接続",
      blocks: [
        "**PostgreSQL**、**MySQL/MariaDB**、**MongoDB**、**SQL Server**、ファイルフォルダー（**Files**）に対応しています。接続の追加・編集・削除は管理者のみ可能です。",
        {
          list: [
            "**ホスト** はマネージャーを実行するサーバー/コンテナから見たアドレスです。コンテナ内の `localhost` はお使いの PC を指し**ません**。",
            "**バックアップ用アカウント**: 読み取り権限で十分です(PostgreSQL の `pg_read_all_data`、MongoDB の `backup` ロールなど)。スーパーユーザーは避けてください。",
            "**[[connForm.restoreUser]]**(任意): データベース作成権限を持つ別アカウント。空欄にするとこの接続のリストアは無効になります。",
            "**[[settings.check]]** はバックアップ用ログインとリストア用アカウントの DB 作成権限を確認します。",
            "パスワードはサーバー上で暗号化され、ブラウザに返されることはありません。編集時に空欄のままにすると現在の値を保持します。",
            "**SQL Server** は SQL Server とマネージャーコンテナの共有フォルダーが必要です。SQL Server 側のパスとコンテナ内のパスの両方を入力します。",
            "**Files** はコンテナにマウントしたフォルダー（アップロードフォルダーなど）を `.tar.gz` でバックアップします。**[[connForm.sourceDir]]** と、リストアする場合は **[[connForm.restoreDir]]** を入力します。リストアは常に新しいサブフォルダーに展開され、上書きしません。",
          ],
        },
        "実行中のジョブがある接続は編集・削除できません。接続を削除しても既存のバックアップと履歴は残ります。",
      ],
    },
    {
      id: "backups",
      title: "バックアップ・ダウンロード・アップロード",
      blocks: [
        {
          list: [
            "接続ページまたは **[[nav.allBackups]]** の **[[action.backupNow]]** を使用します。各バックアップは保存前に検証(形式 + SHA-256)されます。",
            "ファイル形式: PostgreSQL `.dump`、MySQL `.sql.gz`、MongoDB `.archive.gz`、SQL Server `.bak`、Files `.tar.gz`。",
            "接続とデータベースで絞り込めます。件数とサイズはフィルターに連動します。",
            "**[[action.upload]]** で既存のバックアップを取り込みます(既定で最大 10 GiB)。選択したデータベースの種類と同じ形式である必要があります。",
            "手動およびアップロードしたバックアップは削除するまで保持されます。保持期間による削除はスケジュール分のみです。",
            "ジョブは 1 件ずつ実行され、画面は 5 秒ごとに更新されます。",
          ],
        },
      ],
    },
    {
      id: "restore",
      title: "リストア",
      blocks: [
        "リストアは常に **新しいデータベース** を作成し、既存のデータベースを上書きすることはありません。",
        {
          steps: [
            "バックアップのリストアアイコンをクリックします。",
            "リストア先の接続を選びます(同じ種類で、リストア用アカウントが設定済み)。元の接続と異なっても構いません。",
            "新しいデータベース名を入力し、確認のため再入力して、バックアップの提供元を信頼することを確認します。",
            "**[[nav.activity]]** でジョブを確認します。アプリに権限を与える前にデータを確認してください。",
          ],
        },
        {
          note: "途中で失敗した場合、リストア先データベースは調査用に残ります。再試行するには別名を使うか、失敗した DB を手動で削除してください。",
        },
      ],
    },
    {
      id: "schedules",
      title: "スケジュールと保持期間",
      blocks: [
        "スケジュールは 5 フィールドの cron 形式です: `分 時 日 月 曜日`。",
        { code: cronExamples },
        {
          list: [
            "スケジュールはシステムのタイムゾーンで動作します。管理者はアカウントメニュー(右上)→ **[[menu.changeTimezone]]** で変更できます。",
            "**保持期間**(1〜3650 日)はスケジュールで作成したバックアップのみ削除します。スケジュール実行後と毎日 03:17 に実行されます。",
            "複数のスケジュールが同じ DB に該当する場合は最長の保持期間が使われます。スケジュールを無効化・削除するとその DB の保持処理も止まります。",
            "アプリ停止中に逃した実行は、起動時に**1 回だけ補完実行**されます (複数回逃しても 1 回のバックアップ)。",
            "失敗したバックアップは失敗として通知される前に**自動で再試行**されます (既定 2 回)。",
            "スケジュールにクラウドの保存先を紐付けると、バックアップを自動でアップロードします。",
          ],
        },
      ],
    },
    {
      id: "cloud",
      title: "クラウドストレージと暗号化",
      blocks: [
        "rclone 経由で Google Drive、OneDrive、Dropbox、S3/R2/MinIO、B2、SFTP、WebDAV などに対応します。ファイルはアップロード前に**サーバー上で暗号化**され、クラウドには `.age` ファイルのみが送られます。",
        {
          steps: [
            "Google Drive: **[[googleOAuth.option]]** を選択して **[[googleOAuth.connect]]** をクリックし、Google で認証してから保存します。README に従いサーバーで OAuth を設定してください。SSH トンネルの localhost を使えるためサーバーの公開は不要です。",
            "手動設定・他のクラウド: `rclone config` を実行し、`rclone config show <リモート名>` の出力を **[[destForm.config]]** に貼り付けます。S3 の設定は直接入力できます。",
            "暗号化方式を選びます: **[[destForm.publicKeyRec]]**— **[[destForm.genKey]]** をクリックして秘密鍵を保存、または **パスフレーズ**。",
            "**[[destForm.test]]** をクリックし、クラウド上でテストファイルの書き込み・読み取り・削除を確認します。",
          ],
        },
        {
          note: "秘密鍵は**一度だけ**表示され、サーバーには保存されません。サーバーやクラウドアカウントとは別の場所に保管してください。鍵を失うとバックアップも失われます。",
        },
        "復元するには **[[nav.cloud]] → [[cloud.browse]]** でファイルを選び、秘密鍵を入力します。復号されたコピーはソース *[[source.cloud]]* としてバックアップ一覧に表示され、通常どおりリストアできます。サーバーを失った場合の手動復号:",
        { code: ageDecrypt },
      ],
    },
    {
      id: "users",
      title: "ユーザーと権限",
      blocks: [
        {
          list: [
            "**管理者**: すべての権限 — 接続、クラウド保存先、ユーザー、全データベース。",
            "**ユーザー**: 付与された範囲のみ表示されます。各権限は *接続* × *データベース*(空欄 = すべて)× *レベル* です。",
            "**閲覧** レベル: バックアップ・スケジュール・ジョブの閲覧。**操作** レベル: バックアップ、ダウンロード、アップロード、スケジュール管理、バックアップ削除、クラウド送信。",
            "*[[common.allDatabases]]* のバックアップ/スケジュールとリストアには、その接続の**全**データベースに対する操作権限が必要です。",
            "各ユーザーはアカウントメニュー → **[[menu.profile]]** で自分のパスワードを変更できます。",
          ],
        },
        "管理者パスワードを忘れた場合: サーバーで以下のコマンドを実行し、`.env` の `ADMIN_PASSWORD` でログインします。",
        { code: resetAdmin },
      ],
    },
    {
      id: "notifications",
      title: "通知とリストアテスト",
      blocks: [
        "管理者は **[[nav.notifications]] → [[notify.add]]** から Telegram、Slack、Discord、メール、Webhook への通知を設定できます。保存前に **[[notifyForm.test]]** で確認してください。",
        {
          list: [
            "**[[notify.event.failure]]**: バックアップ、リストア、リストアテスト、クラウドアップロードの失敗 (バックアップは最後の再試行後)。",
            "**[[notify.event.stale]]**: スケジュール時刻を 15 分過ぎてもジョブが作成されない場合 (コンテナ停止中など)。",
            "**[[notify.event.connection]]**: 5 分ごとにすべての接続を確認し、2 回連続で失敗したときと復旧したときに通知します。",
            "**[[notify.event.overdue]]**: `BACKUP_MAX_AGE` (既定 24h、スケジュール間隔 + `JOB_TIMEOUT` の方が長ければそちら) を超えて成功したバックアップがない場合。",
            "**[[notify.event.backup_size]]**: 新しいバックアップが前回より `BACKUP_SHRINK_PERCENT` (既定 50%) 以上小さい場合。",
            "**[[notify.event.destination]]**: クラウド保存先のチェック (30 分ごと) が 2 回連続で失敗したときと復旧したとき。",
            "**[[notify.event.disk]]**: バックアップフォルダーのディスク空き容量が `DISK_ALERT_PERCENT` (既定 10%) 未満になった場合。",
            "**[[notify.event.restore]]**: リストアが要求された場合。",
            "**[[notify.event.security]]**: ある IP アドレスのログインが 10 回失敗し、15 分間ロックされた場合。",
            "**[[notify.event.success]]**: ジョブ完了 – 通知が多くなるため任意です。",
          ],
        },
        "**リストアテスト** はバックアップを一時データベース `dbbm_verify_…` にリストアし、テーブル数を確認して削除します。各バックアップのフラスコボタンで手動実行するか、スケジュール作成時に *[[schedule.verify]]* を有効にします。",
        {
          note: "テストはテストサーバーのリソースとディスクを使用します。大きなデータベースでは本番ではなくステージングサーバーを選んでください。",
        },
      ],
    },
    {
      id: "troubleshooting",
      title: "トラブルシューティング",
      blocks: [
        {
          list: [
            "**接続できない**: マネージャーコンテナから見たホスト/ポート(同じ Docker ネットワーク)、ファイアウォール、SSL モード、認証情報を確認してください。",
            "**ジョブが失敗した**: **[[nav.activity]]** タブでジョブのエラーを確認します。権限エラーはバックアップ/リストア用アカウントの権限不足が主な原因です。",
            "**PostgreSQL のバージョンエラー**: イメージ内のクライアント(`PG_MAJOR`)はサーバーと同じかそれ以上である必要があります。",
            "**SQL Server が .bak を読めない**: コンテナを SQL Server のグループに追加(`group_add`)し、共有フォルダーにスティッキービットを使わないでください。",
            "**アップロードが拒否された**: ファイル形式がデータベースの種類と一致し、サイズが上限以下か確認してください。",
            "**再起動時に実行中だったジョブ** は失敗として記録され、自動再実行されません。手動で再実行してください。",
          ],
        },
      ],
    },
  ],
};

/**
 * Renders **bold**, *italic* and `code` inside a line of text. [[key]] is
 * replaced by that UI label so the guide always matches the buttons.
 */
function inline(raw: string): ReactNode {
  const s = raw.replace(/\[\[([\w.]+)\]\]/g, (_, k: string) => t(k as Key));
  return s.split(/(\*\*[^*]+\*\*|`[^`]+`|\*[^*]+\*)/g).map((part, i) => {
    if (part.startsWith("**") && part.endsWith("**"))
      return <strong key={i}>{part.slice(2, -2)}</strong>;
    if (part.startsWith("`") && part.endsWith("`"))
      return <code key={i}>{part.slice(1, -1)}</code>;
    if (part.length > 2 && part.startsWith("*") && part.endsWith("*"))
      return <em key={i}>{part.slice(1, -1)}</em>;
    return <Fragment key={i}>{part}</Fragment>;
  });
}

function renderBlock(b: Block, i: number) {
  if (typeof b === "string") return <p key={i}>{inline(b)}</p>;
  if ("list" in b)
    return (
      <ul key={i}>
        {b.list.map((x, j) => (
          <li key={j}>{inline(x)}</li>
        ))}
      </ul>
    );
  if ("steps" in b)
    return (
      <ol key={i}>
        {b.steps.map((x, j) => (
          <li key={j}>{inline(x)}</li>
        ))}
      </ol>
    );
  if ("code" in b)
    return (
      <pre key={i}>
        <code>{b.code}</code>
      </pre>
    );
  return (
    <div key={i} className="notice help-note">
      {inline(b.note)}
    </div>
  );
}

export function HelpPage({ section }: { section?: string }) {
  const sections = content[getLang()];
  useEffect(() => {
    if (!section) return;
    document
      .getElementById("help-" + section)
      ?.scrollIntoView({ behavior: "smooth", block: "start" });
  }, [section]);
  return (
    <>
      <div className="page-head">
        <div>
          <h1>{t("nav.help")}</h1>
          <p>{t("help.subtitle")}</p>
        </div>
      </div>
      <div className="help-layout">
        <nav className="card help-toc" aria-label={t("help.contents")}>
          <div className="nav-label">{t("help.contents")}</div>
          {sections.map((s) => {
            const Icon = icons[s.id] ?? LifeBuoy;
            return (
              <a
                key={s.id}
                href={"#/help/" + s.id}
                className={"nav-item" + (section === s.id ? " active" : "")}
              >
                <Icon size={16} />
                <span>{s.title}</span>
              </a>
            );
          })}
        </nav>
        <div className="help-body">
          {sections.map((s) => {
            const Icon = icons[s.id] ?? LifeBuoy;
            return (
              <section
                key={s.id}
                id={"help-" + s.id}
                className="card help-section"
              >
                <h2>
                  <Icon size={18} /> {s.title}
                </h2>
                {s.blocks.map(renderBlock)}
              </section>
            );
          })}
        </div>
      </div>
    </>
  );
}
