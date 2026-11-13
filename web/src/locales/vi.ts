// Source dictionary. Other locales must define every key (checked by tsc).
// Syntax: {name} = variable, {n|one|other} = plural form,
// <b>…</b> / <code>…</code> / <tag>…</tag> = rich text (only with tx()).
const vi = {
  // common
  "common.loading": "Đang tải…",
  "common.cancel": "Hủy",
  "common.close": "Đóng",
  "common.edit": "Sửa",
  "common.delete": "Xóa",
  "common.change": "Đổi",
  "common.retry": "Thử lại",
  "pager.range": "{from}–{to} / {total}",
  "pager.page": "Trang {n}/{total}",
  "pager.prev": "Trang trước",
  "pager.next": "Trang sau",
  "common.viewAll": "Xem tất cả",
  "common.none": "Chưa có",
  "common.no": "Không",
  "common.menu": "Menu",
  "common.version": "Phiên bản",
  "common.databases": "Databases",
  "common.database": "Database",
  "common.backups": "Backups",
  "common.backup": "Backup",
  "common.restore": "Restore",
  "common.connection": "Kết nối",
  "common.allConnections": "Tất cả kết nối",
  "common.allDatabases": "Tất cả database",
  "common.lastBackup": "Backup gần nhất",
  "common.schedules": "Lịch",
  "common.host": "Host",
  "common.account": "Tài khoản",
  "common.size": "Dung lượng",
  "common.time": "Thời gian",
  "common.status": "Trạng thái",
  "common.active": "Hoạt động",
  "common.notConfigured": "Chưa cấu hình",
  "common.keepBlank": "Để trống để giữ nguyên",
  "common.optional": "tùy chọn",
  "common.saveChanges": "Lưu thay đổi",
  "common.language": "Ngôn ngữ",
  "error.request": "Không thể hoàn thành yêu cầu",

  // time
  "time.justNow": "vừa xong",
  "time.minutesAgo": "{n} phút trước",
  "time.hoursAgo": "{n} giờ trước",
  "time.daysAgo": "{n} ngày trước",

  // job status / kinds / backup sources
  "status.completed": "Hoàn tất",
  "status.running": "Đang chạy",
  "status.queued": "Đang chờ",
  "status.failed": "Thất bại",
  "job.offsite": "Upload lên cloud",
  "job.offsite_pull": "Tải về từ cloud",
  "job.restore": "Restore",
  "job.retention": "Dọn dẹp retention",
  "job.scheduled": "Backup tự động",
  "job.backup": "Backup",
  "source.scheduled": "Tự động",
  "source.upload": "Upload",
  "source.cloud": "Từ cloud",
  "source.manual": "Thủ công",

  // navigation
  "nav.overview": "Tổng quan",
  "nav.databases": "Databases",
  "nav.backups": "Backups",
  "nav.schedules": "Lịch tự động",
  "nav.activity": "Hoạt động",
  "nav.settings": "Cài đặt",
  "nav.connections": "Kết nối",
  "nav.allBackups": "Tất cả backup",
  "nav.cloud": "Lưu trữ cloud",
  "nav.activityLog": "Nhật ký hoạt động",
  "nav.users": "Người dùng",
  "nav.account": "Tài khoản",
  "nav.help": "Hướng dẫn",
  "help.subtitle":
    "Cách dùng DB Backup Manager: kết nối, backup, restore, lịch chạy, cloud và phân quyền.",
  "help.contents": "Nội dung",
  "topbar.jobsRunning": "{n} job đang chạy",

  // login
  "login.title": "Đăng nhập",
  "login.subtitle": "Quản lý backup cho các máy chủ cơ sở dữ liệu của bạn.",
  "login.username": "Tên đăng nhập",
  "login.password": "Mật khẩu",
  "login.submit": "Đăng nhập",

  // actions
  "action.backupNow": "Backup ngay",
  "action.addSchedule": "Thêm lịch backup",
  "action.recheck": "Kiểm tra lại",
  "action.createBackup": "Tạo bản backup",
  "action.uploadBackup": "Upload backup",
  "action.upload": "Upload",
  "action.download": "Download",
  "menu.open": "Mở",
  "menu.actions": "Thao tác",

  // connections
  "conn.add": "Thêm kết nối",
  "conn.edit": "Sửa kết nối",
  "conn.delete": "Xóa kết nối",
  "conn.deleted": "Kết nối đã xóa",
  "conn.env": "Kết nối từ env",
  "conn.deleteConfirm":
    "Xóa kết nối <b>{name}</b>? Các bản backup đã lưu vẫn được giữ lại. Cần xóa lịch liên quan và đợi job hoàn tất trước khi xóa kết nối.",
  "home.subtitle": "Các máy chủ database bạn đang quản lý backup.",
  "home.search": "Tìm theo tên hoặc host…",
  "home.filterAll": "Tất cả ({n})",
  "home.filterOk": "Hoạt động ({n})",
  "home.filterError": "Lỗi ({n})",
  "home.schedulesOn": "{n} đang bật",
  "home.noResults": "Không có kết quả",
  "home.noResultsText": "Thử từ khóa hoặc bộ lọc khác.",
  "home.empty": "Chưa có kết nối",
  "home.emptyText": "Thêm máy chủ database đầu tiên để bắt đầu backup.",
  "probe.checking": "Đang kiểm tra",
  "probe.down": "Mất kết nối",

  // backups
  "backups.search": "Tìm database…",
  "backups.count": "{n} bản · {size}",
  "backups.restoreHint":
    "Để restore, vào Cài đặt của kết nối đích và cấu hình tài khoản restore.",
  "backups.noMatch": "Không có bản backup phù hợp",
  "backups.empty": "Chưa có bản backup",
  "backups.emptyText": "Tạo backup hoặc upload một file backup có sẵn.",
  "backups.emptyNoAccess":
    "Bạn chưa có bản backup nào trong phạm vi được cấp quyền.",
  "backups.subtitle": "Bản backup của mọi kết nối trong tài khoản.",
  "backups.source": "Nguồn",
  "backups.cloudCopies": "Bản mã hóa trên: {list}",
  "backups.toCloud": "Mã hóa và upload lên cloud",
  "backups.restore": "Restore sang database mới",
  "backups.restoreDisabled":
    "Cấu hình tài khoản restore trong Cài đặt của kết nối",
  "backups.delete": "Xóa backup",

  // schedules
  "schedules.tzNotice":
    "Lịch chạy theo <b>{tz}</b>. Retention chỉ xóa bản backup tự động; bản thủ công và upload được giữ lại.",
  "schedules.empty": "Chưa có lịch backup",
  "schedules.emptyText": "Thêm lịch backup và thời gian lưu trữ.",
  "schedules.subtitle": "Lịch backup của mọi kết nối.",
  "schedules.nextRun": "Chạy tiếp theo",
  "schedules.retention": "Lưu trữ",
  "schedules.days": "{n} ngày",
  "schedules.enabled": "Đang bật",
  "schedules.paused": "Tạm dừng",
  "schedules.pause": "Tạm dừng",
  "schedules.enable": "Bật lịch",
  "schedules.edit": "Sửa lịch",
  "schedules.delete": "Xóa lịch",

  // jobs & audit
  "jobs.empty": "Chưa có hoạt động",
  "jobs.emptyText":
    "Các job backup, restore và retention sẽ được ghi lại tại đây.",
  "jobs.started": "Bắt đầu",
  "jobs.finished": "Kết thúc",
  "jobs.errorDetail": "Chi tiết lỗi",
  "activity.subtitle": "200 job gần nhất · tự cập nhật mỗi 5 giây",
  "activity.jobs": "Job",
  "activity.audit": "Nhật ký thao tác",
  "activity.auditAdmin":
    "Đăng nhập, download, upload, xóa, thay đổi lịch · 100 sự kiện gần nhất",
  "activity.auditUser": "Thao tác của bạn · 100 sự kiện gần nhất",
  "activity.auditEmpty": "Chưa có sự kiện",
  "activity.auditEmptyText": "Thao tác của bạn sẽ được lưu tại đây.",
  "audit.event": "Sự kiện",
  "audit.actor": "Người thực hiện",
  "audit.detail": "Chi tiết",
  "audit.system": "Hệ thống",

  // connection overview
  "overview.dataSize": "{size} dữ liệu",
  "overview.notConnected": "Chưa kết nối",
  "overview.backups": "Bản backup",
  "overview.stored": "{size} lưu trữ",
  "overview.activeSchedules": "Lịch đang bật",
  "overview.scheduleCount": "{n} lịch · {tz}",
  "overview.recentBackups": "Backup gần đây",
  "overview.backupsEmptyText":
    "Bản backup sẽ xuất hiện tại đây sau khi hoàn tất.",
  "overview.recentActivity": "Hoạt động gần đây",
  "overview.quickActions": "Thao tác nhanh",
  "overview.connInfo": "Thông tin kết nối",
  "overview.connSettings": "Cài đặt kết nối",
  "dbs.loadError": "Không lấy được danh sách database",
  "dbs.connecting": "Đang kết nối…",
  "dbs.checking": "Đang kiểm tra kết nối tới máy chủ database.",
  "dbs.empty": "Không có database",
  "dbs.emptyText": "Tài khoản này không thấy database nào trên máy chủ.",
  "zone.notFound": "Không tìm thấy kết nối",
  "zone.notFoundText": "Kết nối này có thể đã bị xóa.",
  "zone.back": "Về danh sách kết nối",
  "zone.pgError": "Không kết nối được máy chủ database.",

  // connection settings
  "settings.server": "Máy chủ",
  "settings.serverText":
    "{addr} · tài khoản <b>{user}</b> · database <b>{db}</b> · SSL {ssl}",
  "settings.hasPassword": "có mật khẩu",
  "settings.noPassword": "không mật khẩu",
  "settings.restoreOn":
    "Bật với tài khoản <b>{user}</b>. Restore luôn tạo database mới trên máy chủ này.",
  "settings.restoreOff":
    "Chưa cấu hình tài khoản restore. Backup từ kết nối khác không thể restore vào máy chủ này.",
  "settings.configure": "Cấu hình",
  "settings.check": "Kiểm tra kết nối",
  "settings.currentStatus": "Trạng thái hiện tại:",
  "settings.deleteText":
    "Các bản backup đã lưu vẫn được giữ lại. Cần xóa lịch liên quan và đợi job hoàn tất trước khi xóa.",

  // cloud
  "cloud.subtitle":
    "Bản backup được mã hóa trên máy chủ trước khi upload lên Google Drive, OneDrive, S3…",
  "cloud.noRclone":
    "Không tìm thấy <code>rclone</code> trên máy chủ backup. Build lại Docker image hoặc cài rclone để dùng lưu trữ cloud.",
  "cloud.encryption": "Mã hóa",
  "cloud.copies": "Bản sao",
  "cloud.latest": "Gần nhất",
  "cloud.keep": "Giữ",
  "cloud.keepDays": "{n} ngày (luôn giữ bản mới nhất)",
  "cloud.forever": "Vĩnh viễn",
  "cloud.scheduleUse": "{n} lịch dùng đích này",
  "cloud.browse": "Duyệt file & tải về",
  "cloud.empty": "Chưa có đích cloud",
  "cloud.emptyText":
    "Thêm Google Drive, OneDrive, S3… để tự động gửi bản backup đã mã hóa ra ngoài máy chủ.",
  "cloud.decryptNote":
    "Giải mã thủ công khi mất máy chủ: <code>age -d -i key.txt file.dump.age > file.dump</code> (hoặc <code>age -d file.dump.age</code> và nhập passphrase), rồi restore bằng công cụ của database (<code>pg_restore</code>, <code>mariadb</code>, <code>mongorestore</code> hoặc <code>RESTORE DATABASE</code>). Retention trên cloud chạy cùng lượt dọn dẹp hằng ngày lúc 03:17.",
  "dest.add": "Thêm đích cloud",
  "dest.edit": "Sửa đích cloud",
  "dest.delete": "Xóa đích cloud",
  "dest.deleted": "Đích đã xóa",
  "dest.deleteConfirm":
    "Xóa đích <b>{name}</b>? File đã upload vẫn nằm trên cloud nhưng ứng dụng sẽ không quản lý hay dọn dẹp chúng nữa. Cần gỡ đích khỏi các lịch backup trước.",

  // users
  "users.adminRequired": "Cần quyền quản trị",
  "users.adminRequiredText": "Chỉ quản trị viên mới quản lý được người dùng.",
  "users.subtitle": "Tài khoản đăng nhập và phân quyền theo kết nối, database.",
  "users.add": "Thêm người dùng",
  "users.edit": "Sửa người dùng",
  "users.delete": "Xóa người dùng",
  "users.user": "Người dùng",
  "users.role": "Vai trò",
  "users.permissions": "Quyền",
  "users.createdAt": "Tạo lúc",
  "users.fullAccess": "Toàn quyền",
  "users.locked": "Đã khóa",
  "users.empty": "Chưa có người dùng",
  "users.emptyText":
    "Tạo tài khoản cho đồng nghiệp và cấp quyền theo kết nối hoặc từng database.",
  "users.deleteConfirm":
    "Xóa người dùng <b>{name}</b>? Tài khoản bị đăng xuất ngay và mất toàn bộ quyền. Bản backup và lịch vẫn được giữ.",
  "role.admin": "Quản trị viên",
  "role.viewer": "Chỉ xem",
  "role.user": "Người dùng",
  "level.view": "Xem",
  "level.operate": "Vận hành",
  "grants.none": "Chưa được cấp quyền",
  "grants.allDbs": "tất cả DB",

  // user menu & profile
  "menu.timezone": "Múi giờ",
  "menu.profile": "Hồ sơ & mật khẩu",
  "menu.changeTimezone": "Đổi múi giờ lịch chạy",
  "menu.users": "Quản lý người dùng",
  "menu.logout": "Đăng xuất",
  "profile.title": "Hồ sơ",
  "profile.changePassword": "Đổi mật khẩu",
  "profile.viewerNote":
    "Tài khoản viewer: đổi mật khẩu bằng <code>VIEWER_PASSWORD</code> trong file <code>.env</code> rồi khởi động lại dịch vụ.",
  "pw.current": "Mật khẩu hiện tại",
  "pw.new": "Mật khẩu mới",
  "pw.hint": "12–72 ký tự. Các phiên đăng nhập khác sẽ bị đăng xuất.",
  "pw.confirm": "Nhập lại mật khẩu mới",
  "pw.mismatch": "Mật khẩu không khớp",

  // timezone modal
  "tz.title": "Múi giờ lịch chạy",
  "tz.label": "Múi giờ (IANA)",
  "tz.default": "Mặc định từ <code>TZ</code> trong .env: {tz}",
  "tz.note":
    "Mọi lịch backup và lượt dọn dẹp 03:17 sẽ chạy theo múi giờ mới ngay sau khi lưu. Biểu thức cron giữ nguyên, chỉ đổi cách hiểu giờ.",
  "tz.reset": "Về mặc định",
  "tz.save": "Lưu múi giờ",

  // modals: shared pieces
  "modal.chooseDb": "Chọn database",
  "modal.noUpload": "Không upload",
  "modal.destHint": "Mã hóa bằng age rồi upload sau khi backup hoàn tất.",
  "modal.destHintEmpty":
    "Thêm đích trong mục Lưu trữ cloud để gửi bản backup ra ngoài.",
  "modal.cloudFiles": "File trên {name}",

  // backup / restore / delete
  "backup.intro":
    "Tạo bản backup đầy đủ của database và xác minh checksum SHA-256 sau khi hoàn tất.",
  "backup.start": "Bắt đầu backup",
  "backup.delete": "Xóa bản backup",
  "backup.deleteConfirm":
    "Xóa vĩnh viễn bản backup <b>{db}</b> ({date}, {size}) khỏi máy chủ?",
  "restore.title": "Khôi phục database",
  "restore.notice":
    "Restore luôn tạo <b>database mới</b>. Tên đích phải chưa tồn tại.",
  "restore.target": "Kết nối đích",
  "restore.notConfigured": "Chưa cấu hình restore",
  "restore.account": "Tài khoản restore: {user}",
  "restore.targetName": "Tên database đích",
  "restore.confirmName": "Nhập lại tên đích để xác nhận",
  "restore.trust":
    "Tôi tin cậy nguồn archive này và đồng ý thực thi nội dung SQL khi restore.",
  "restore.submit": "Xác nhận restore",

  // schedule modal
  "schedule.daily": "Hàng ngày · 02:00",
  "schedule.every6h": "Mỗi 6 giờ",
  "schedule.sunday": "Chủ nhật · 02:00",
  "schedule.cron": "Biểu thức cron",
  "schedule.cronHint": "phút · giờ · ngày · tháng · thứ — {tz}",
  "schedule.retention": "Lưu trữ (ngày)",
  "schedule.sendCloud": "Gửi bản sao lên cloud",
  "schedule.note":
    "Nếu nhiều lịch trùng database, áp dụng thời gian lưu dài nhất. Xóa quá hạn sau lượt chạy và lúc 03:17 mỗi ngày. Bản trên cloud dùng thời gian lưu riêng của đích cloud.",
  "schedule.save": "Lưu lịch",

  // upload modal
  "upload.title": "Upload bản backup",
  "upload.chooseFile": "Chọn file backup",
  "upload.intro":
    "Nhận file {format}. Giới hạn {size}. Chỉ upload file từ nguồn bạn tin cậy.",
  "upload.dbName": "Tên database gốc",
  "upload.pick": "Chọn file ({ext})",
  "upload.submit": "Upload & xác minh",

  // to-cloud modal
  "toCloud.dest": "Đích cloud",
  "toCloud.exists": "(đã có — upload lại)",
  "toCloud.note":
    "Archive được kiểm tra SHA-256, mã hóa bằng age trên máy chủ rồi mới upload; cloud chỉ nhận file đã mã hóa (.dump.age).",
  "toCloud.submit": "Mã hóa & upload",

  // cloud files / pull modal
  "pull.intro":
    "Tải bản mã hóa về máy chủ, giải mã và kiểm tra archive. Bản tải về xuất hiện trong danh sách backup (nguồn “{source}”) để restore.",
  "pull.loading": "Đang đọc danh sách file…",
  "pull.onServer": "đã có trên máy chủ",
  "pull.empty": "Chưa có file",
  "pull.emptyText": "Không có file .dump.age trong /{path}",
  "pull.dbName": "Database gốc",
  "pull.privateKey": "Private key age",
  "pull.otherPassphrase": "Passphrase khác (tùy chọn)",
  "pull.passphrasePlaceholder": "Để trống để dùng passphrase đã lưu",
  "pull.secretHint":
    "Chỉ giữ trong bộ nhớ đến khi job giải mã xong; không lưu vào cơ sở dữ liệu.",
  "pull.submit": "Tải về & giải mã",

  // toasts
  "notice.loggedOut": "Đã đăng xuất",
  "notice.scheduleUpdated": "Đã cập nhật lịch",
  "notice.scheduleDeleted": "Đã xóa lịch backup",
  "notice.scheduleCreated": "Đã tạo lịch backup",
  "notice.tzChanged": "Đã đổi múi giờ lịch chạy",
  "notice.passwordChanged": "Đã đổi mật khẩu",
  "notice.userSaved": "Đã lưu người dùng",
  "notice.userCreated": "Đã tạo người dùng",
  "notice.userDeleted": "Đã xóa người dùng",
  "notice.connSaved": "Đã lưu kết nối",
  "notice.connDeleted": "Đã xóa kết nối",
  "notice.backupQueued": "Đã đưa backup vào hàng đợi",
  "notice.restoreQueued": "Đã đưa restore vào hàng đợi",
  "notice.uploaded": "Upload và xác minh archive thành công",
  "notice.destSaved": "Đã lưu đích cloud",
  "notice.destDeleted": "Đã xóa đích cloud",
  "notice.backupDeleted": "Đã xóa bản backup",
  "notice.offsiteQueued": "Đã đưa upload cloud vào hàng đợi",
  "notice.pullQueued": "Đã đưa job tải về vào hàng đợi",

  // connection form
  "connForm.testOkRestore":
    "Kết nối backup và quyền của tài khoản restore hợp lệ",
  "connForm.testOk": "Kết nối {engine} thành công",
  "connForm.name": "Tên kết nối",
  "connForm.namePlaceholder": "Ví dụ: Production, Staging",
  "connForm.hostPlaceholder": "db.example.com hoặc IP",
  "connForm.clearPassword": "Xóa mật khẩu đã lưu (khi để trống ô mật khẩu)",
  "connForm.database": "Database kết nối",
  "connForm.databaseHint": "Dùng để lấy danh sách DB. Mặc định: postgres.",
  "connForm.restoreUser": "Tài khoản restore",
  "connForm.restorePassword": "Mật khẩu restore",
  "connForm.restoreHint":
    "Để trống để tắt restore. Cần LOGIN, CREATEDB và quyền kết nối database maintenance trên máy chủ này. Restore chỉ tạo DB mới; object thuộc tài khoản restore.",
  "connForm.clearRestorePassword":
    "Xóa mật khẩu restore đã lưu (khi để trống ô mật khẩu)",
  "connForm.hostNote":
    "Host phải truy cập được từ máy chủ chạy backup; localhost là chính máy chủ hoặc container đó.",

  // rclone provider help
  "help.other": "Khác",
  "help.drive1":
    "Trên máy có trình duyệt, cài <rclone>rclone</rclone> và chạy <code>rclone config</code> → <code>n</code> (new remote) → chọn <code>drive</code>.",
  "help.drive2":
    "Scope nên chọn <code>drive.file</code> (rclone chỉ thấy file do nó tạo). Đăng nhập Google khi trình duyệt mở ra.",
  "help.drive3":
    "Chạy <code>rclone config show <tên-remote></code> rồi dán toàn bộ kết quả vào ô bên dưới.",
  "help.onedrive1":
    "Trên máy có trình duyệt, chạy <code>rclone config</code> → <code>n</code> → chọn <code>onedrive</code>, đăng nhập Microsoft.",
  "help.onedrive2":
    "Chọn loại OneDrive (Personal / Business / SharePoint) và drive khi rclone hỏi.",
  "help.onedrive3":
    "Chạy <code>rclone config show <tên-remote></code> rồi dán toàn bộ kết quả vào ô bên dưới. Token được tự gia hạn và lưu lại sau mỗi lần chạy.",
  "help.s31":
    "Tạo access key chỉ có quyền ghi/đọc/xóa trên bucket dành cho backup.",
  "help.s32":
    "Điền cấu hình theo mẫu (provider: <code>AWS</code>, <code>Cloudflare</code>, <code>Minio</code>, <code>Wasabi</code>…). Thư mục bên dưới bắt đầu bằng tên bucket.",
  "help.otherText":
    "Hỗ trợ mọi <link>backend của rclone</link> (Dropbox, Box, B2, SFTP, WebDAV, pCloud…). Tạo remote bằng <code>rclone config</code> rồi dán kết quả <code>rclone config show <tên-remote></code>.",

  // destination form
  "keyfile.title": 'Private key giải mã backup cho đích "{name}"',
  "keyfile.decrypt": "Giải mã",
  "destForm.passMismatch": "Passphrase nhập lại không khớp",
  "destForm.confirmKeySaved":
    "Xác nhận bạn đã lưu private key trước khi lưu đích cloud",
  "destForm.testOk": "Ghi, đọc và xóa file kiểm tra trên cloud thành công",
  "destForm.storage": "Nơi lưu trữ",
  "destForm.name": "Tên",
  "destForm.namePlaceholder": "Ví dụ: Google Drive công ty",
  "destForm.config": "Cấu hình rclone",
  "destForm.configKeep": "Để trống để giữ cấu hình {provider} đã lưu",
  "destForm.configHint":
    "Mã hóa bằng khóa của máy chủ trước khi lưu; không bao giờ trả về trình duyệt.",
  "destForm.remotePath": "Thư mục trên cloud",
  "destForm.remotePathHint": "File lưu tại <thư mục>/<kết nối>/<database>/",
  "destForm.keepDays": "Giữ (ngày)",
  "destForm.forever": "0 = vĩnh viễn",
  "destForm.publicKeyRec": "Public key (khuyến nghị)",
  "destForm.passphrase": "Passphrase",
  "destForm.recipientNote":
    "Backup được mã hóa bằng <b>age</b> (X25519 + ChaCha20-Poly1305). Máy chủ chỉ giữ public key nên <b>kể cả khi máy chủ hoặc tài khoản cloud bị lộ, file backup vẫn không đọc được</b>. Cần private key để tải về và giải mã.",
  "destForm.publicKey": "Public key age",
  "destForm.publicKeyPlaceholder":
    "age1… (mỗi dòng một khóa; có thể thêm khóa dự phòng)",
  "destForm.genKey": "Tạo cặp khóa mới",
  "destForm.privateOnce": "Private key — chỉ hiển thị một lần",
  "destForm.copy": "Sao chép",
  "destForm.downloadTxt": "Tải file .txt",
  "destForm.privateHint":
    "Máy chủ không lưu private key. Cất ở nơi an toàn, tách khỏi máy chủ và tài khoản cloud (ví dụ trình quản lý mật khẩu). Mất private key = không khôi phục được backup.",
  "destForm.keySaved": "Tôi đã lưu private key ở nơi an toàn.",
  "destForm.passphraseNote":
    "Mã hóa age với passphrase (scrypt). Passphrase được lưu mã hóa trên máy chủ để upload tự động — ai chiếm được máy chủ cũng có thể giải mã. Hãy lưu passphrase ở nơi khác để khôi phục khi mất máy chủ.",
  "destForm.minChars": "Tối thiểu 16 ký tự",
  "destForm.repeat": "Nhập lại",
  "destForm.rekeyHint":
    "Đổi khóa chỉ áp dụng cho các lần upload sau; file cũ trên cloud vẫn cần khóa cũ để giải mã.",
  "destForm.test": "Kiểm tra",

  // user form
  "userForm.displayName": "Tên hiển thị",
  "userForm.displayPlaceholder": "Ví dụ: Nguyễn Văn A",
  "userForm.resetPassword": "Đặt lại mật khẩu",
  "userForm.pwLength": "12–72 ký tự",
  "userForm.resetHint": "Đặt lại mật khẩu sẽ đăng xuất người dùng này.",
  "userForm.userText":
    "Chỉ thấy và thao tác trên kết nối, database được cấp quyền.",
  "userForm.adminText":
    "Toàn quyền: kết nối, cloud, người dùng và mọi database.",
  "userForm.lock": "Khóa tài khoản (không thể đăng nhập)",
  "userForm.grants": "Phân quyền",
  "userForm.level": "Mức quyền",
  "userForm.removeGrant": "Bỏ quyền",
  "userForm.addGrant": "Thêm quyền",
  "userForm.create": "Tạo người dùng",

  // database engines
  "connForm.engine": "Loại database",
  "connForm.authDatabase": "Database xác thực",
  "connForm.databaseHintMongo":
    "Database chứa tài khoản (authSource). Mặc định: admin.",
  "connForm.databaseHintMssql":
    "Dùng để đăng nhập và lấy danh sách DB. Mặc định: master.",
  "connForm.mongoNoAuth": "Để trống nếu MongoDB không bật xác thực.",
  "connForm.sharedDir": "Thư mục backup dùng chung",
  "connForm.serverDir": "Đường dẫn phía SQL Server",
  "connForm.localDir": "Đường dẫn trong container",
  "connForm.sharedDirHint":
    "SQL Server ghi file .bak vào thư mục này bằng lệnh BACKUP DATABASE. Container backup phải mount đúng thư mục đó (volume chung, SMB hoặc NFS) với quyền đọc/ghi, và SQL Server phải đọc được file container ghi vào.",
  "connForm.restoreHintMysql":
    "Để trống để tắt restore. Cần quyền CREATE toàn cục và toàn quyền trên database mới tạo. Restore chỉ tạo DB mới; DEFINER của view/routine được bỏ để object thuộc tài khoản restore.",
  "connForm.restoreHintMongo":
    "Để trống để tắt restore. Cần vai trò restore (hoặc readWrite trên database đích) và quyền listDatabases. Restore chỉ ghi vào DB chưa tồn tại.",
  "connForm.restoreHintMssql":
    "Để trống để tắt restore. Cần vai trò dbcreator (CREATE ANY DATABASE). Restore chỉ tạo DB mới, file dữ liệu đặt ở thư mục mặc định của SQL Server.",
  "restore.noTarget": "Chưa có kết nối {engine} nào bật restore",
  // restore tests, retries and notifications
  "job.verify": "Kiểm thử restore",
  "nav.notifications": "Thông báo",
  "jobs.attempt": "Lần thử {n}",
  "verify.action": "Kiểm thử restore",
  "verify.title": "Kiểm thử restore",
  "verify.passed": "Kiểm thử restore đạt",
  "verify.failed": "Kiểm thử restore thất bại",
  "verify.badgeOk": "Restore OK",
  "verify.badgeFail": "Restore lỗi",
  "verify.scheduleBadge": "Kiểm thử",
  "verify.intro":
    "Bản backup sẽ được restore vào một <b>database tạm mới</b> (tên bắt đầu bằng <code>dbbm_verify_</code>), kiểm tra mở được và đếm số bảng, rồi <b>xóa database tạm</b>. Không database nào khác bị chạm tới. Việc này dùng CPU, RAM và dung lượng đĩa của Server kiểm thử trong lúc chạy.",
  "verify.server": "Server kiểm thử",
  "verify.serverHint":
    "Dùng tài khoản restore của Server này. Nên chọn Server staging nếu database lớn.",
  "verify.submit": "Chạy kiểm thử",
  "schedule.verify": "Tự kiểm thử restore sau mỗi lần backup",
  "schedule.verifyHint":
    "Restore bản mới vào database tạm, kiểm tra rồi xóa. Kết quả hiện cạnh bản backup và được gửi qua kênh thông báo.",
  "schedule.verifyUnavailable":
    "Cần ít nhất một kết nối cùng loại database có bật tài khoản restore.",
  "schedule.verifyServer": "Server kiểm thử",
  "schedule.verifyServerHint":
    "Kiểm thử tạo và xóa database trên Server này, nên chọn staging nếu có.",
  "schedule.sameServer": "cùng Server",
  "schedule.verifyOn": "Kiểm thử restore sau mỗi lần backup trên {server}",
  "notice.verifyQueued": "Đã xếp hàng kiểm thử restore",
  "notice.notifierSaved": "Đã lưu kênh thông báo",
  "notice.notifierDeleted": "Đã xóa kênh thông báo",
  "notify.subtitle":
    "Báo qua Telegram, Slack, Discord, email hoặc webhook khi backup lỗi hoặc lịch không chạy.",
  "notify.add": "Thêm kênh",
  "notify.edit": "Sửa kênh thông báo",
  "notify.delete": "Xóa kênh thông báo",
  "notify.deleteConfirm":
    "Xóa kênh <b>{name}</b>? Kênh này sẽ không nhận thông báo nữa.",
  "notify.events": "Sự kiện",
  "notify.on": "Đang bật",
  "notify.off": "Đang tắt",
  "notify.urlHidden": "URL được ẩn",
  "notify.empty": "Chưa có kênh thông báo",
  "notify.emptyText":
    "Thêm một kênh để biết ngay khi backup thất bại, thay vì phải mở trang này để kiểm tra.",
  "notify.note":
    "Token, webhook URL và mật khẩu SMTP được mã hóa trong metadata và không bao giờ hiển thị lại. Job lỗi được <b>tự thử lại</b> trước khi báo (cấu hình bằng <code>JOB_RETRIES</code>, <code>JOB_RETRY_DELAY</code>).",
  "notify.event.failure": "Job thất bại",
  "notify.event.stale": "Lịch không chạy",
  "notify.event.connection": "Mất kết nối DB",
  "notify.event.overdue": "Lâu không có backup",
  "notify.event.backup_size": "Backup nhỏ bất thường",
  "notify.event.destination": "Cloud không truy cập được",
  "notify.event.disk": "Ổ đĩa sắp đầy",
  "notify.event.restore": "Có yêu cầu restore",
  "notify.event.security": "Đăng nhập sai nhiều lần",
  "notify.event.success": "Job hoàn tất",
  "notify.eventHint.failure":
    "Backup, restore, kiểm thử restore, upload cloud hoặc dọn bản cũ bị lỗi (sau khi đã thử lại).",
  "notify.eventHint.stale":
    "Đến giờ chạy theo lịch nhưng không có job nào được tạo, ví dụ khi container bị dừng.",
  "notify.eventHint.connection":
    "Kiểm tra kết nối mỗi 5 phút; báo khi lỗi 2 lần liên tiếp và báo lại khi kết nối khôi phục.",
  "notify.eventHint.overdue":
    "Lịch đang bật nhưng quá BACKUP_MAX_AGE (mặc định 24h) chưa có bản backup thành công, kể cả khi job vẫn chạy và lỗi liên tục.",
  "notify.eventHint.backup_size":
    "Bản backup mới nhỏ hơn bản trước quá BACKUP_SHRINK_PERCENT (mặc định 50%). Dấu hiệu dữ liệu bị thiếu hoặc bị xóa.",
  "notify.eventHint.destination":
    "Kiểm tra nơi lưu cloud mỗi 30 phút; báo khi lỗi 2 lần liên tiếp và báo lại khi hoạt động lại.",
  "notify.eventHint.disk":
    "Dung lượng trống của thư mục backup dưới DISK_ALERT_PERCENT (mặc định 10%), và khi đủ chỗ trở lại.",
  "notify.eventHint.restore":
    "Ngay khi có người yêu cầu restore, kèm tên người thực hiện và database đích.",
  "notify.eventHint.security":
    "Một địa chỉ IP đăng nhập sai 10 lần và bị khóa 15 phút.",
  "notify.eventHint.success":
    "Backup, restore và kiểm thử restore thành công. Có thể khá nhiều tin.",
  "notifyForm.name": "Tên kênh",
  "notifyForm.namePlaceholder": "Ví dụ: Nhóm vận hành",
  "notifyForm.botToken": "Bot token",
  "notifyForm.chatId": "Chat ID",
  "notifyForm.webhookUrl": "Webhook URL",
  "notifyForm.hmacSecret": "Secret ký HMAC",
  "notifyForm.optional": "Không bắt buộc",
  "notifyForm.smtpHost": "Máy chủ SMTP",
  "notifyForm.smtpUser": "Tài khoản SMTP",
  "notifyForm.smtpPassword": "Mật khẩu SMTP",
  "notifyForm.from": "Người gửi",
  "notifyForm.to": "Người nhận",
  "notifyForm.security": "Bảo mật kết nối",
  "notifyForm.noTls": "Không mã hóa (chỉ dùng trong mạng nội bộ)",
  "notifyForm.events": "Gửi khi",
  "notifyForm.lang": "Ngôn ngữ tin nhắn",
  "notify.pause": "Tạm tắt",
  "notify.resume": "Bật lại",
  "notice.notifierPaused": "Đã tạm tắt kênh thông báo",
  "notice.notifierResumed": "Đã bật lại kênh thông báo",
  "notifyForm.test": "Gửi thử",
  "notifyForm.testOk": "Đã gửi tin thử. Kiểm tra kênh nhận.",
  "notifyForm.help.telegram":
    "1. Chat với <b>@BotFather</b>, gửi <code>/newbot</code> để lấy bot token. 2. Thêm bot vào nhóm (hoặc nhắn cho bot một tin). 3. Lấy Chat ID: mở <code>https://api.telegram.org/bot<token>/getUpdates</code>, tìm <code>chat.id</code> (nhóm có dạng <code>-100…</code>).",
  "notifyForm.help.slack":
    "Trong Slack: <b>Apps → Incoming Webhooks</b>, chọn kênh và sao chép Webhook URL (<code>https://hooks.slack.com/services/…</code>).",
  "notifyForm.help.discord":
    "Trong Discord: <b>Cài đặt kênh → Tích hợp → Webhook → Webhook mới</b>, sao chép URL.",
  "notifyForm.help.webhook":
    "Gửi POST JSON gồm <code>event</code>, <code>title</code>, <code>text</code>, <code>connection</code>, <code>database</code> và <code>job</code>. Nếu có secret, header <code>X-DBBM-Signature: sha256=…</code> là HMAC-SHA256 của body. Dùng được với Microsoft Teams Workflows, n8n, Zapier…",
  "notifyForm.help.email":
    "Gmail/Google Workspace: <code>smtp.gmail.com</code>, STARTTLS 587, dùng <b>App password</b> thay vì mật khẩu chính. Nhiều người nhận cách nhau bằng dấu phẩy.",
  // branding
  "menu.branding": "Thương hiệu",
  "branding.title": "Tên và logo",
  "branding.name": "Tên hiển thị",
  "branding.nameHint":
    "Hiện ở thanh trên cùng, trang đăng nhập và tiêu đề tab. Để trống để dùng “{name}”.",
  "branding.pick": "Chọn ảnh logo (PNG, JPEG, WebP, GIF, SVG)",
  "branding.logoHint":
    "Tối đa 256 KB. Nên dùng ảnh vuông, nền trong suốt; logo cũng được dùng làm favicon.",
  "branding.removeLogo": "Dùng logo mặc định",
  "branding.tooLarge": "Logo tối đa 256 KB",
  "notice.brandingSaved": "Đã lưu tên và logo",
  // files engine
  "connForm.sourceDir": "Thư mục nguồn (trong container)",
  "connForm.sourceDirHint":
    "Thư mục cần backup, đã mount vào container (nên mount chỉ đọc), ví dụ /sources/uploads.",
  "connForm.itemName": "Tên mục backup",
  "connForm.itemNameHint":
    "Tên đại diện cho thư mục này trong danh sách backup, lịch, phân quyền và tên file tải về (ví dụ uploads-….tar.gz). Tự lấy theo tên thư mục nguồn; nên giữ nguyên sau khi đã có backup.",
  "connForm.restoreDir": "Thư mục restore (trong container)",
  "connForm.restoreDirHint":
    "Bản backup được giải nén vào một thư mục con mới bên trong thư mục này, không bao giờ ghi đè. Cần quyền ghi cho UID 999. Để trống để tắt restore.",
  "connForm.filesNote":
    "Container chỉ đọc được các thư mục đã mount vào nó. Thêm volume trong docker-compose, ví dụ: /var/www/app/uploads:/sources/uploads:ro",
  "connForm.testOkFiles": "Đọc được thư mục nguồn",
  "common.folder": "Thư mục",
  "restore.folder": "Giải nén vào: {dir}",
  "restore.targetFolder": "Tên thư mục đích (thư mục mới)",
  "settings.filesText": "Thư mục nguồn {addr} · mục backup <b>{db}</b>",
  "settings.restoreDir": "Restore vào thư mục <b>{user}</b>",
  // Google Drive web OAuth
  "googleOAuth.option": "Đăng nhập bằng Google OAuth",
  "googleOAuth.description":
    "Đăng nhập Google để cấp quyền cho thư mục backup. Token được lưu trên máy chủ; không cần cài rclone trên máy cá nhân.",
  "googleOAuth.setup":
    "Quản trị viên cần cấu hình GOOGLE_OAUTH_CLIENT_ID, GOOGLE_OAUTH_CLIENT_SECRET và GOOGLE_OAUTH_REDIRECT_URL trên máy chủ. Xem hướng dẫn Google OAuth trong README.",
  "googleOAuth.callback": "URL callback đã đăng ký với Google:",
  "googleOAuth.tunnel":
    "Qua SSH tunnel: mở web bằng localhost và đúng cổng trong URL callback. Server không cần public Internet.",
  "googleOAuth.ready":
    "Đã cấp quyền Google Drive. Bấm Lưu để giữ kết nối này (trong 30 phút).",
  "googleOAuth.connect": "Kết nối Google Drive",
  "googleOAuth.reconnect": "Kết nối lại Google Drive",
  "googleOAuth.waiting": "Đang chờ cấp quyền Google…",
  "googleOAuth.popup": "Cho phép trình duyệt mở cửa sổ đăng nhập rồi thử lại.",
  "googleOAuth.required": "Kết nối Google Drive trước khi kiểm tra hoặc lưu.",
  "googleOAuth.scope": "Quyền truy cập Drive",
  "googleOAuth.scopeFile": "Chỉ file do ứng dụng này tạo (khuyến nghị)",
  "googleOAuth.scopeFull": "Toàn bộ Drive — dùng thư mục hoặc backup cũ",
  "googleOAuth.scopeHint":
    "Nếu chuyển từ OAuth Client khác và cần đọc backup cũ, chọn toàn bộ Drive rồi kết nối lại. Quyền này cho phép ứng dụng đọc và sửa mọi file Drive.",
};

export default vi;
