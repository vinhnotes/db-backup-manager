import type vi from "./vi";

const ja: Record<keyof typeof vi, string> = {
  // common
  "common.loading": "読み込み中…",
  "common.cancel": "キャンセル",
  "common.close": "閉じる",
  "common.edit": "編集",
  "common.delete": "削除",
  "common.change": "変更",
  "common.retry": "再試行",
  "pager.range": "{total}件中 {from}–{to}",
  "pager.page": "{n} / {total} ページ",
  "pager.prev": "前のページ",
  "pager.next": "次のページ",
  "common.viewAll": "すべて表示",
  "common.none": "まだありません",
  "common.no": "なし",
  "common.menu": "メニュー",
  "common.version": "バージョン",
  "common.databases": "データベース",
  "common.database": "データベース",
  "common.backups": "バックアップ",
  "common.backup": "バックアップ",
  "common.restore": "リストア",
  "common.connection": "接続",
  "common.allConnections": "すべての接続",
  "common.allDatabases": "すべてのデータベース",
  "common.lastBackup": "最新のバックアップ",
  "common.schedules": "スケジュール",
  "common.host": "ホスト",
  "common.account": "アカウント",
  "common.size": "サイズ",
  "common.time": "日時",
  "common.status": "状態",
  "common.active": "稼働中",
  "common.notConfigured": "未設定",
  "common.keepBlank": "空欄のままで現在の値を維持",
  "common.optional": "任意",
  "common.saveChanges": "変更を保存",
  "common.language": "言語",
  "error.request": "リクエストを完了できませんでした",

  // time
  "time.justNow": "たった今",
  "time.minutesAgo": "{n}分前",
  "time.hoursAgo": "{n}時間前",
  "time.daysAgo": "{n}日前",

  // job status / kinds / backup sources
  "status.completed": "完了",
  "status.running": "実行中",
  "status.queued": "待機中",
  "status.failed": "失敗",
  "job.offsite": "クラウドへアップロード",
  "job.offsite_pull": "クラウドからダウンロード",
  "job.restore": "リストア",
  "job.retention": "保持期間のクリーンアップ",
  "job.scheduled": "自動バックアップ",
  "job.backup": "バックアップ",
  "source.scheduled": "自動",
  "source.upload": "アップロード",
  "source.cloud": "クラウドから",
  "source.manual": "手動",

  // navigation
  "nav.overview": "概要",
  "nav.databases": "データベース",
  "nav.backups": "バックアップ",
  "nav.schedules": "スケジュール",
  "nav.activity": "アクティビティ",
  "nav.settings": "設定",
  "nav.connections": "接続",
  "nav.allBackups": "すべてのバックアップ",
  "nav.cloud": "クラウドストレージ",
  "nav.activityLog": "アクティビティログ",
  "nav.users": "ユーザー",
  "nav.account": "アカウント",
  "nav.help": "ヘルプ",
  "help.subtitle":
    "DB Backup Manager の使い方: 接続、バックアップ、リストア、スケジュール、クラウド、権限。",
  "help.contents": "目次",
  "topbar.jobsRunning": "{n}件のジョブを実行中",

  // login
  "login.title": "ログイン",
  "login.subtitle": "データベースサーバーのバックアップを管理します。",
  "login.username": "ユーザー名",
  "login.password": "パスワード",
  "login.submit": "ログイン",

  // actions
  "action.backupNow": "今すぐバックアップ",
  "action.addSchedule": "スケジュールを追加",
  "action.recheck": "再確認",
  "action.createBackup": "バックアップを作成",
  "action.uploadBackup": "バックアップをアップロード",
  "action.upload": "アップロード",
  "action.download": "ダウンロード",
  "menu.open": "開く",
  "menu.actions": "操作",

  // connections
  "conn.add": "接続を追加",
  "conn.edit": "接続を編集",
  "conn.delete": "接続を削除",
  "conn.deleted": "削除された接続",
  "conn.env": "env の接続",
  "conn.deleteConfirm":
    "接続 <b>{name}</b> を削除しますか？保存済みのバックアップは残ります。削除する前に関連するスケジュールを削除し、ジョブの完了を待ってください。",
  "home.subtitle": "バックアップを管理しているデータベースサーバー。",
  "home.search": "名前またはホストで検索…",
  "home.filterAll": "すべて ({n})",
  "home.filterOk": "稼働中 ({n})",
  "home.filterError": "エラー ({n})",
  "home.schedulesOn": "{n}件有効",
  "home.noResults": "該当なし",
  "home.noResultsText": "別のキーワードやフィルターをお試しください。",
  "home.empty": "接続がありません",
  "home.emptyText":
    "最初のデータベースサーバーを追加してバックアップを始めましょう。",
  "probe.checking": "確認中",
  "probe.down": "接続不可",

  // backups
  "backups.search": "データベースを検索…",
  "backups.count": "{n}件 · {size}",
  "backups.restoreHint":
    "リストアするには、対象の接続の設定でリストア用アカウントを設定してください。",
  "backups.noMatch": "該当するバックアップがありません",
  "backups.empty": "バックアップがありません",
  "backups.emptyText":
    "バックアップを作成するか、既存のバックアップファイルをアップロードしてください。",
  "backups.emptyNoAccess": "権限のある範囲にバックアップはまだありません。",
  "backups.subtitle": "このアカウントのすべての接続のバックアップ。",
  "backups.source": "取得元",
  "backups.cloudCopies": "暗号化コピーの保存先: {list}",
  "backups.toCloud": "暗号化してクラウドへアップロード",
  "backups.restore": "新しいデータベースへリストア",
  "backups.restoreDisabled":
    "接続の設定でリストア用アカウントを設定してください",
  "backups.delete": "バックアップを削除",

  // schedules
  "schedules.tzNotice":
    "スケジュールは <b>{tz}</b> で実行されます。保持期間による削除は自動バックアップのみが対象で、手動・アップロードしたバックアップは残ります。",
  "schedules.empty": "バックアップスケジュールがありません",
  "schedules.emptyText":
    "バックアップスケジュールと保持期間を追加してください。",
  "schedules.subtitle": "すべての接続のバックアップスケジュール。",
  "schedules.nextRun": "次回実行",
  "schedules.retention": "保持期間",
  "schedules.days": "{n}日",
  "schedules.enabled": "有効",
  "schedules.paused": "一時停止中",
  "schedules.pause": "一時停止",
  "schedules.enable": "スケジュールを有効化",
  "schedules.edit": "スケジュールを編集",
  "schedules.delete": "スケジュールを削除",

  // jobs & audit
  "jobs.empty": "アクティビティがありません",
  "jobs.emptyText":
    "バックアップ、リストア、保持期間のジョブがここに記録されます。",
  "jobs.started": "開始",
  "jobs.finished": "終了",
  "jobs.errorDetail": "エラーの詳細",
  "activity.subtitle": "直近 200 件のジョブ · 5 秒ごとに自動更新",
  "activity.jobs": "ジョブ",
  "activity.audit": "操作ログ",
  "activity.auditAdmin":
    "ログイン、ダウンロード、アップロード、削除、スケジュール変更 · 直近 100 件",
  "activity.auditUser": "あなたの操作 · 直近 100 件",
  "activity.auditEmpty": "イベントがありません",
  "activity.auditEmptyText": "あなたの操作がここに記録されます。",
  "audit.event": "イベント",
  "audit.actor": "実行者",
  "audit.detail": "詳細",
  "audit.system": "システム",

  // connection overview
  "overview.dataSize": "データ {size}",
  "overview.notConnected": "未接続",
  "overview.backups": "バックアップ",
  "overview.stored": "保存容量 {size}",
  "overview.activeSchedules": "有効なスケジュール",
  "overview.scheduleCount": "{n}件 · {tz}",
  "overview.recentBackups": "最近のバックアップ",
  "overview.backupsEmptyText": "完了したバックアップがここに表示されます。",
  "overview.recentActivity": "最近のアクティビティ",
  "overview.quickActions": "クイック操作",
  "overview.connInfo": "接続情報",
  "overview.connSettings": "接続の設定",
  "dbs.loadError": "データベース一覧を取得できませんでした",
  "dbs.connecting": "接続中…",
  "dbs.checking": "データベースサーバーへの接続を確認しています。",
  "dbs.empty": "データベースがありません",
  "dbs.emptyText": "このアカウントからはサーバー上のデータベースが見えません。",
  "zone.notFound": "接続が見つかりません",
  "zone.notFoundText": "この接続は削除された可能性があります。",
  "zone.back": "接続一覧に戻る",
  "zone.pgError": "データベースサーバーに接続できません。",

  // connection settings
  "settings.server": "サーバー",
  "settings.serverText":
    "{addr} · アカウント <b>{user}</b> · データベース <b>{db}</b> · SSL {ssl}",
  "settings.hasPassword": "パスワードあり",
  "settings.noPassword": "パスワードなし",
  "settings.restoreOn":
    "アカウント <b>{user}</b> で有効。リストアは常にこのサーバー上に新しいデータベースを作成します。",
  "settings.restoreOff":
    "リストア用アカウントが未設定です。他の接続のバックアップをこのサーバーへリストアできません。",
  "settings.configure": "設定",
  "settings.check": "接続テスト",
  "settings.currentStatus": "現在の状態:",
  "settings.deleteText":
    "保存済みのバックアップは残ります。削除する前に関連するスケジュールを削除し、ジョブの完了を待ってください。",

  // cloud
  "cloud.subtitle":
    "バックアップはサーバー上で暗号化されてから Google Drive、OneDrive、S3 などへアップロードされます。",
  "cloud.noRclone":
    "バックアップサーバーに <code>rclone</code> が見つかりません。クラウドストレージを使うには Docker イメージを再ビルドするか rclone をインストールしてください。",
  "cloud.encryption": "暗号化",
  "cloud.copies": "コピー",
  "cloud.latest": "最新",
  "cloud.keep": "保持",
  "cloud.keepDays": "{n}日 (最新のコピーは常に保持)",
  "cloud.forever": "無期限",
  "cloud.scheduleUse": "{n}件のスケジュールで使用中",
  "cloud.browse": "ファイルを参照してダウンロード",
  "cloud.empty": "クラウド保存先がありません",
  "cloud.emptyText":
    "Google Drive、OneDrive、S3 などを追加すると、暗号化したバックアップを自動でサーバー外へ送信できます。",
  "cloud.decryptNote":
    "サーバーを失った場合の手動復号: <code>age -d -i key.txt file.dump.age > file.dump</code> (または <code>age -d file.dump.age</code> を実行してパスフレーズを入力) の後、データベースのツール (<code>pg_restore</code>、<code>mariadb</code>、<code>mongorestore</code>、<code>RESTORE DATABASE</code>) でリストアします。クラウドの保持期間処理は毎日 03:17 のクリーンアップと同時に実行されます。",
  "dest.add": "クラウド保存先を追加",
  "dest.edit": "クラウド保存先を編集",
  "dest.delete": "クラウド保存先を削除",
  "dest.deleted": "削除された保存先",
  "dest.deleteConfirm":
    "保存先 <b>{name}</b> を削除しますか？アップロード済みのファイルはクラウドに残りますが、アプリでは管理・削除されなくなります。先にバックアップスケジュールから保存先を外してください。",

  // users
  "users.adminRequired": "管理者権限が必要です",
  "users.adminRequiredText": "ユーザーを管理できるのは管理者のみです。",
  "users.subtitle": "ログインアカウントと、接続・データベースごとの権限。",
  "users.add": "ユーザーを追加",
  "users.edit": "ユーザーを編集",
  "users.delete": "ユーザーを削除",
  "users.user": "ユーザー",
  "users.role": "ロール",
  "users.permissions": "権限",
  "users.createdAt": "作成日時",
  "users.fullAccess": "すべての権限",
  "users.locked": "ロック中",
  "users.empty": "ユーザーがいません",
  "users.emptyText":
    "チームメンバーのアカウントを作成し、接続またはデータベース単位で権限を付与します。",
  "users.deleteConfirm":
    "ユーザー <b>{name}</b> を削除しますか？アカウントは即座にログアウトされ、すべての権限を失います。バックアップとスケジュールは残ります。",
  "role.admin": "管理者",
  "role.viewer": "閲覧のみ",
  "role.user": "ユーザー",
  "level.view": "閲覧",
  "level.operate": "操作",
  "grants.none": "権限が付与されていません",
  "grants.allDbs": "全DB",

  // user menu & profile
  "menu.timezone": "タイムゾーン",
  "menu.profile": "プロフィールとパスワード",
  "menu.changeTimezone": "スケジュールのタイムゾーンを変更",
  "menu.users": "ユーザー管理",
  "menu.logout": "ログアウト",
  "profile.title": "プロフィール",
  "profile.changePassword": "パスワードを変更",
  "profile.viewerNote":
    "viewer アカウント: <code>.env</code> ファイルの <code>VIEWER_PASSWORD</code> でパスワードを変更し、サービスを再起動してください。",
  "pw.current": "現在のパスワード",
  "pw.new": "新しいパスワード",
  "pw.hint": "12〜72 文字。他のセッションはログアウトされます。",
  "pw.confirm": "新しいパスワード (確認)",
  "pw.mismatch": "パスワードが一致しません",

  // timezone modal
  "tz.title": "スケジュールのタイムゾーン",
  "tz.label": "タイムゾーン (IANA)",
  "tz.default": ".env の <code>TZ</code> による既定値: {tz}",
  "tz.note":
    "保存後すぐに、すべてのバックアップスケジュールと 03:17 のクリーンアップが新しいタイムゾーンで実行されます。cron 式はそのままで、時刻の解釈だけが変わります。",
  "tz.reset": "既定値に戻す",
  "tz.save": "タイムゾーンを保存",

  // modals: shared pieces
  "modal.chooseDb": "データベースを選択",
  "modal.noUpload": "アップロードしない",
  "modal.destHint": "バックアップ完了後に age で暗号化してアップロードします。",
  "modal.destHintEmpty":
    "バックアップをサーバー外へ送るには、クラウドストレージで保存先を追加してください。",
  "modal.cloudFiles": "{name} のファイル",

  // backup / restore / delete
  "backup.intro":
    "データベースの完全バックアップを作成し、完了後に SHA-256 チェックサムを検証します。",
  "backup.start": "バックアップを開始",
  "backup.delete": "バックアップを削除",
  "backup.deleteConfirm":
    "<b>{db}</b> のバックアップ ({date}、{size}) をサーバーから完全に削除しますか？",
  "restore.title": "データベースをリストア",
  "restore.notice":
    "リストアは常に<b>新しいデータベース</b>を作成します。リストア先の名前は既存のものと重複できません。",
  "restore.target": "リストア先の接続",
  "restore.notConfigured": "リストア未設定",
  "restore.account": "リストア用アカウント: {user}",
  "restore.targetName": "リストア先データベース名",
  "restore.confirmName": "確認のためリストア先の名前を再入力",
  "restore.trust":
    "このアーカイブの提供元を信頼し、リストア時に SQL の内容を実行することに同意します。",
  "restore.submit": "リストアを実行",

  // schedule modal
  "schedule.daily": "毎日 · 02:00",
  "schedule.every6h": "6 時間ごと",
  "schedule.sunday": "日曜日 · 02:00",
  "schedule.cron": "cron 式",
  "schedule.cronHint": "分 · 時 · 日 · 月 · 曜日 — {tz}",
  "schedule.retention": "保持期間 (日)",
  "schedule.sendCloud": "コピーをクラウドへ送信",
  "schedule.note":
    "同じデータベースに複数のスケジュールがある場合は、最も長い保持期間が適用されます。期限切れのバックアップは各実行後と毎日 03:17 に削除されます。クラウド上のコピーには保存先ごとの保持期間が適用されます。",
  "schedule.save": "スケジュールを保存",

  // upload modal
  "upload.title": "バックアップをアップロード",
  "upload.chooseFile": "バックアップファイルを選択してください",
  "upload.intro":
    "{format} を受け付けます。上限 {size}。信頼できる提供元のファイルのみアップロードしてください。",
  "upload.dbName": "元のデータベース名",
  "upload.pick": "ファイルを選択 ({ext})",
  "upload.submit": "アップロードして検証",

  // to-cloud modal
  "toCloud.dest": "クラウド保存先",
  "toCloud.exists": "(アップロード済み — 再アップロード)",
  "toCloud.note":
    "アーカイブは SHA-256 で検証され、サーバー上で age により暗号化されてからアップロードされます。クラウドが受け取るのは暗号化済みファイル (.dump.age) のみです。",
  "toCloud.submit": "暗号化してアップロード",

  // cloud files / pull modal
  "pull.intro":
    "暗号化されたコピーをサーバーへダウンロードし、復号してアーカイブを検証します。ダウンロードしたものはバックアップ一覧 (取得元「{source}」) に表示され、リストアできます。",
  "pull.loading": "ファイル一覧を読み込み中…",
  "pull.onServer": "サーバーに存在",
  "pull.empty": "ファイルがありません",
  "pull.emptyText": "/{path} に .dump.age ファイルがありません",
  "pull.dbName": "元のデータベース",
  "pull.privateKey": "age 秘密鍵",
  "pull.otherPassphrase": "別のパスフレーズ (任意)",
  "pull.passphrasePlaceholder": "空欄の場合は保存済みのパスフレーズを使用",
  "pull.secretHint":
    "復号ジョブが終わるまでメモリ上にのみ保持され、データベースには保存されません。",
  "pull.submit": "ダウンロードして復号",

  // toasts
  "notice.loggedOut": "ログアウトしました",
  "notice.scheduleUpdated": "スケジュールを更新しました",
  "notice.scheduleDeleted": "バックアップスケジュールを削除しました",
  "notice.scheduleCreated": "バックアップスケジュールを作成しました",
  "notice.tzChanged": "スケジュールのタイムゾーンを変更しました",
  "notice.passwordChanged": "パスワードを変更しました",
  "notice.userSaved": "ユーザーを保存しました",
  "notice.userCreated": "ユーザーを作成しました",
  "notice.userDeleted": "ユーザーを削除しました",
  "notice.connSaved": "接続を保存しました",
  "notice.connDeleted": "接続を削除しました",
  "notice.backupQueued": "バックアップをキューに追加しました",
  "notice.restoreQueued": "リストアをキューに追加しました",
  "notice.uploaded": "アーカイブをアップロードして検証しました",
  "notice.destSaved": "クラウド保存先を保存しました",
  "notice.destDeleted": "クラウド保存先を削除しました",
  "notice.backupDeleted": "バックアップを削除しました",
  "notice.offsiteQueued": "クラウドへのアップロードをキューに追加しました",
  "notice.pullQueued": "ダウンロードジョブをキューに追加しました",

  // connection form
  "connForm.testOkRestore":
    "バックアップ接続とリストア用アカウントの権限は有効です",
  "connForm.testOk": "{engine} に接続できました",
  "connForm.name": "接続名",
  "connForm.namePlaceholder": "例: Production、Staging",
  "connForm.hostPlaceholder": "db.example.com または IP",
  "connForm.clearPassword":
    "保存済みのパスワードを削除 (パスワード欄が空の場合)",
  "connForm.database": "接続用データベース",
  "connForm.databaseHint":
    "データベース一覧の取得に使用します。既定: postgres。",
  "connForm.restoreUser": "リストア用アカウント",
  "connForm.restorePassword": "リストア用パスワード",
  "connForm.restoreHint":
    "空欄にするとリストアは無効になります。このサーバーで LOGIN、CREATEDB、およびメンテナンス用データベースへの接続権限が必要です。リストアは新しい DB のみを作成し、オブジェクトの所有者はリストア用アカウントになります。",
  "connForm.clearRestorePassword":
    "保存済みのリストア用パスワードを削除 (パスワード欄が空の場合)",
  "connForm.hostNote":
    "ホストはバックアップを実行するサーバーから到達可能である必要があります。localhost はそのサーバーまたはコンテナ自身を指します。",

  // rclone provider help
  "help.other": "その他",
  "help.drive1":
    "ブラウザのあるマシンで <rclone>rclone</rclone> をインストールし、<code>rclone config</code> → <code>n</code> (new remote) → <code>drive</code> を選択します。",
  "help.drive2":
    "スコープは <code>drive.file</code> を推奨します (rclone は自身が作成したファイルのみ参照)。ブラウザが開いたら Google にログインします。",
  "help.drive3":
    "<code>rclone config show <リモート名></code> を実行し、出力全体を下の欄に貼り付けます。",
  "help.onedrive1":
    "ブラウザのあるマシンで <code>rclone config</code> → <code>n</code> → <code>onedrive</code> を選択し、Microsoft にログインします。",
  "help.onedrive2":
    "rclone に聞かれたら OneDrive の種類 (Personal / Business / SharePoint) とドライブを選択します。",
  "help.onedrive3":
    "<code>rclone config show <リモート名></code> を実行し、出力全体を下の欄に貼り付けます。トークンは自動で更新され、実行のたびに保存されます。",
  "help.s31":
    "バックアップ用バケットへの読み取り・書き込み・削除のみ許可したアクセスキーを作成します。",
  "help.s32":
    "テンプレートに従って設定を記入します (provider: <code>AWS</code>、<code>Cloudflare</code>、<code>Minio</code>、<code>Wasabi</code> など)。下のフォルダはバケット名から始めます。",
  "help.otherText":
    "<link>rclone のバックエンド</link>すべてに対応しています (Dropbox、Box、B2、SFTP、WebDAV、pCloud など)。<code>rclone config</code> でリモートを作成し、<code>rclone config show <リモート名></code> の出力を貼り付けます。",

  // destination form
  "keyfile.title": "保存先「{name}」のバックアップ復号用秘密鍵",
  "keyfile.decrypt": "復号",
  "destForm.passMismatch": "パスフレーズが一致しません",
  "destForm.confirmKeySaved":
    "保存先を保存する前に、秘密鍵を保管したことを確認してください",
  "destForm.testOk":
    "クラウド上でテストファイルの書き込み・読み取り・削除に成功しました",
  "destForm.storage": "保存先",
  "destForm.name": "名前",
  "destForm.namePlaceholder": "例: 社内 Google Drive",
  "destForm.config": "rclone 設定",
  "destForm.configKeep": "空欄のままで保存済みの {provider} 設定を維持",
  "destForm.configHint":
    "保存前にサーバーの鍵で暗号化され、ブラウザに返されることはありません。",
  "destForm.remotePath": "クラウド上のフォルダ",
  "destForm.remotePathHint":
    "ファイルの保存先: <フォルダ>/<接続>/<データベース>/",
  "destForm.keepDays": "保持 (日)",
  "destForm.forever": "0 = 無期限",
  "destForm.publicKeyRec": "公開鍵 (推奨)",
  "destForm.passphrase": "パスフレーズ",
  "destForm.recipientNote":
    "バックアップは <b>age</b> (X25519 + ChaCha20-Poly1305) で暗号化されます。サーバーは公開鍵のみを保持するため、<b>サーバーやクラウドアカウントが漏えいしてもバックアップファイルは読み取れません</b>。ダウンロードと復号には秘密鍵が必要です。",
  "destForm.publicKey": "age 公開鍵",
  "destForm.publicKeyPlaceholder":
    "age1… (1 行に 1 つ。予備の鍵も追加できます)",
  "destForm.genKey": "新しい鍵ペアを生成",
  "destForm.privateOnce": "秘密鍵 — 一度だけ表示されます",
  "destForm.copy": "コピー",
  "destForm.downloadTxt": ".txt をダウンロード",
  "destForm.privateHint":
    "サーバーは秘密鍵を保存しません。サーバーやクラウドアカウントとは別の安全な場所 (パスワードマネージャーなど) に保管してください。秘密鍵を失うとバックアップを復元できません。",
  "destForm.keySaved": "秘密鍵を安全な場所に保管しました。",
  "destForm.passphraseNote":
    "パスフレーズ (scrypt) による age 暗号化です。自動アップロードのためパスフレーズはサーバー上に暗号化して保存されるため、サーバーを乗っ取った者も復号できます。サーバーを失っても復元できるよう、パスフレーズは別の場所にも保管してください。",
  "destForm.minChars": "16 文字以上",
  "destForm.repeat": "再入力",
  "destForm.rekeyHint":
    "鍵の変更は今後のアップロードにのみ適用されます。クラウド上の既存ファイルの復号には古い鍵が必要です。",
  "destForm.test": "テスト",

  // user form
  "userForm.displayName": "表示名",
  "userForm.displayPlaceholder": "例: 山田 太郎",
  "userForm.resetPassword": "パスワードを再設定",
  "userForm.pwLength": "12〜72 文字",
  "userForm.resetHint":
    "パスワードを再設定すると、このユーザーはログアウトされます。",
  "userForm.userText":
    "権限を付与された接続・データベースのみ閲覧・操作できます。",
  "userForm.adminText":
    "すべての権限: 接続、クラウド、ユーザー、すべてのデータベース。",
  "userForm.lock": "アカウントをロック (ログイン不可)",
  "userForm.grants": "権限",
  "userForm.level": "権限レベル",
  "userForm.removeGrant": "権限を削除",
  "userForm.addGrant": "権限を追加",
  "userForm.create": "ユーザーを作成",

  // database engines
  "connForm.engine": "データベースの種類",
  "connForm.authDatabase": "認証データベース",
  "connForm.databaseHintMongo":
    "ユーザーが登録されているデータベース (authSource)。既定: admin。",
  "connForm.databaseHintMssql":
    "ログインとデータベース一覧の取得に使用します。既定: master。",
  "connForm.mongoNoAuth": "MongoDB でアクセス制御が無効な場合は空欄にします。",
  "connForm.sharedDir": "共有バックアップディレクトリ",
  "connForm.serverDir": "SQL Server 側のパス",
  "connForm.localDir": "コンテナ内のパス",
  "connForm.sharedDirHint":
    "SQL Server は BACKUP DATABASE でここに .bak ファイルを書き込みます。バックアップコンテナは同じディレクトリ (共有ボリューム、SMB、NFS) を読み書き可能でマウントし、SQL Server はコンテナが書いたファイルを読める必要があります。",
  "connForm.restoreHintMysql":
    "空欄にするとリストアは無効になります。グローバルな CREATE 権限と新しいデータベースへの全権限が必要です。リストアは新しい DB のみを作成し、DEFINER は削除されオブジェクトはリストア用アカウントのものになります。",
  "connForm.restoreHintMongo":
    "空欄にするとリストアは無効になります。restore ロール (またはリストア先の readWrite) と listDatabases が必要です。リストアは存在しない DB にのみ書き込みます。",
  "connForm.restoreHintMssql":
    "空欄にするとリストアは無効になります。dbcreator (CREATE ANY DATABASE) が必要です。リストアは新しい DB のみを作成し、データファイルは SQL Server の既定フォルダーに置かれます。",
  "restore.noTarget": "リストアが有効な {engine} 接続がありません",
  // restore tests, retries and notifications
  "job.verify": "リストアテスト",
  "nav.notifications": "通知",
  "jobs.attempt": "{n} 回目の試行",
  "verify.action": "リストアをテスト",
  "verify.title": "リストアテスト",
  "verify.passed": "リストアテスト成功",
  "verify.failed": "リストアテスト失敗",
  "verify.badgeOk": "リストア OK",
  "verify.badgeFail": "リストア失敗",
  "verify.scheduleBadge": "テスト",
  "verify.intro":
    "バックアップを<b>新しい一時データベース</b> (名前は <code>dbbm_verify_…</code>) にリストアし、開けることとテーブル数を確認した後、<b>一時データベースを削除</b>します。他のデータベースには一切触れません。実行中はテストサーバーの CPU、メモリ、ディスクを使用します。",
  "verify.server": "テストサーバー",
  "verify.serverHint":
    "このサーバーのリストア用アカウントを使用します。大きなデータベースではステージングサーバーを推奨します。",
  "verify.submit": "テストを実行",
  "schedule.verify": "バックアップのたびにリストアをテスト",
  "schedule.verifyHint":
    "新しいバックアップを一時データベースにリストアして確認し、削除します。結果はバックアップの横に表示され、通知チャネルにも送信されます。",
  "schedule.verifyUnavailable":
    "同じ種類のデータベースで、リストア用アカウントが設定された接続が必要です。",
  "schedule.verifyServer": "テストサーバー",
  "schedule.verifyServerHint":
    "テストではこのサーバー上でデータベースを作成・削除します。可能ならステージングを選んでください。",
  "schedule.sameServer": "同じサーバー",
  "schedule.verifyOn": "バックアップごとに {server} でリストアをテスト",
  "notice.verifyQueued": "リストアテストをキューに追加しました",
  "notice.notifierSaved": "通知チャネルを保存しました",
  "notice.notifierDeleted": "通知チャネルを削除しました",
  "notify.subtitle":
    "バックアップの失敗やスケジュール未実行を Telegram、Slack、Discord、メール、Webhook で通知します。",
  "notify.add": "チャネルを追加",
  "notify.edit": "チャネルを編集",
  "notify.delete": "チャネルを削除",
  "notify.deleteConfirm":
    "<b>{name}</b> を削除しますか? このチャネルには通知が届かなくなります。",
  "notify.events": "イベント",
  "notify.on": "有効",
  "notify.off": "無効",
  "notify.urlHidden": "URL は非表示",
  "notify.empty": "通知チャネルがありません",
  "notify.emptyText":
    "チャネルを追加すると、この画面を開かなくてもバックアップの失敗をすぐに知ることができます。",
  "notify.note":
    "トークン、Webhook URL、SMTP パスワードはメタデータ内で暗号化され、再表示されません。失敗したジョブは通知前に<b>自動で再試行</b>されます (<code>JOB_RETRIES</code>、<code>JOB_RETRY_DELAY</code> で設定)。",
  "notify.event.failure": "ジョブ失敗",
  "notify.event.stale": "スケジュール未実行",
  "notify.event.connection": "DB 接続断",
  "notify.event.overdue": "長時間バックアップなし",
  "notify.event.backup_size": "バックアップが異常に小さい",
  "notify.event.destination": "クラウドに接続不可",
  "notify.event.disk": "ディスク容量不足",
  "notify.event.restore": "リストア要求",
  "notify.event.security": "ログイン失敗の多発",
  "notify.event.success": "ジョブ完了",
  "notify.eventHint.failure":
    "バックアップ、リストア、リストアテスト、クラウドアップロード、削除処理が (再試行後も) 失敗した場合。",
  "notify.eventHint.stale":
    "スケジュールの時刻を過ぎてもジョブが作成されなかった場合 (コンテナ停止中など)。",
  "notify.eventHint.connection":
    "5 分ごとに接続を確認し、2 回連続で失敗したときと、接続が復旧したときに通知します。",
  "notify.eventHint.overdue":
    "有効なスケジュールで BACKUP_MAX_AGE (既定 24h) を超えて成功したバックアップがない場合。ジョブが失敗し続けている場合も含みます。",
  "notify.eventHint.backup_size":
    "新しいバックアップが前回より BACKUP_SHRINK_PERCENT (既定 50%) 以上小さい場合。データの欠落や削除の兆候です。",
  "notify.eventHint.destination":
    "30 分ごとにクラウド保存先を確認し、2 回連続で失敗したときと復旧したときに通知します。",
  "notify.eventHint.disk":
    "バックアップフォルダーの空き容量が DISK_ALERT_PERCENT (既定 10%) を下回ったときと、回復したとき。",
  "notify.eventHint.restore":
    "リストアが要求された時点で、実行者とリストア先データベースを通知します。",
  "notify.eventHint.security":
    "ある IP アドレスからのログインが 10 回失敗し、15 分間ロックされた場合。",
  "notify.eventHint.success":
    "バックアップ、リストア、リストアテストの成功。通知が多くなる場合があります。",
  "notifyForm.name": "チャネル名",
  "notifyForm.namePlaceholder": "例: 運用チーム",
  "notifyForm.botToken": "Bot トークン",
  "notifyForm.chatId": "Chat ID",
  "notifyForm.webhookUrl": "Webhook URL",
  "notifyForm.hmacSecret": "HMAC 署名シークレット",
  "notifyForm.optional": "任意",
  "notifyForm.smtpHost": "SMTP サーバー",
  "notifyForm.smtpUser": "SMTP ユーザー名",
  "notifyForm.smtpPassword": "SMTP パスワード",
  "notifyForm.from": "送信者",
  "notifyForm.to": "宛先",
  "notifyForm.security": "接続のセキュリティ",
  "notifyForm.noTls": "暗号化なし (社内ネットワークのみ)",
  "notifyForm.events": "送信するタイミング",
  "notifyForm.lang": "メッセージの言語",
  "notify.pause": "一時停止",
  "notify.resume": "再開",
  "notice.notifierPaused": "通知チャネルを一時停止しました",
  "notice.notifierResumed": "通知チャネルを再開しました",
  "notifyForm.test": "テスト送信",
  "notifyForm.testOk":
    "テストメッセージを送信しました。受信先を確認してください。",
  "notifyForm.help.telegram":
    "1. <b>@BotFather</b> に <code>/newbot</code> を送って Bot トークンを取得します。2. Bot をグループに追加します (または Bot に一度メッセージを送ります)。3. <code>https://api.telegram.org/bot<token>/getUpdates</code> の <code>chat.id</code> で Chat ID を確認します (グループは <code>-100…</code>)。",
  "notifyForm.help.slack":
    "Slack で <b>Apps → Incoming Webhooks</b> を開き、チャンネルを選んで Webhook URL (<code>https://hooks.slack.com/services/…</code>) をコピーします。",
  "notifyForm.help.discord":
    "Discord で <b>チャンネル設定 → 連携サービス → ウェブフック → 新しいウェブフック</b> を開き、URL をコピーします。",
  "notifyForm.help.webhook":
    "<code>event</code>、<code>title</code>、<code>text</code>、<code>connection</code>、<code>database</code>、<code>job</code> を含む JSON を POST します。シークレットを設定すると <code>X-DBBM-Signature: sha256=…</code> に本文の HMAC-SHA256 が入ります。Microsoft Teams Workflows、n8n、Zapier などで利用できます。",
  "notifyForm.help.email":
    "Gmail/Google Workspace: <code>smtp.gmail.com</code>、STARTTLS 587、アカウントのパスワードではなく<b>アプリ パスワード</b>を使用します。複数の宛先はカンマで区切ります。",
  // branding
  "menu.branding": "ブランディング",
  "branding.title": "名前とロゴ",
  "branding.name": "表示名",
  "branding.nameHint":
    "上部バー、ログイン画面、ブラウザのタブに表示されます。空欄にすると「{name}」になります。",
  "branding.pick": "ロゴ画像を選択 (PNG、JPEG、WebP、GIF、SVG)",
  "branding.logoHint":
    "最大 256 KB。透明背景の正方形画像が最適です。ファビコンにも使用されます。",
  "branding.removeLogo": "既定のロゴに戻す",
  "branding.tooLarge": "ロゴは 256 KB 以下にしてください",
  "notice.brandingSaved": "名前とロゴを保存しました",
  // files engine
  "connForm.sourceDir": "バックアップ元フォルダー（コンテナ内）",
  "connForm.sourceDirHint":
    "コンテナにマウントしたバックアップ対象フォルダー（読み取り専用を推奨）。例: /sources/uploads",
  "connForm.itemName": "バックアップ項目名",
  "connForm.itemNameHint":
    "バックアップ一覧、スケジュール、権限、ダウンロードファイル名（例: uploads-….tar.gz）でこのフォルダーを示す名前です。バックアップ元フォルダー名から自動設定されます。バックアップ作成後は変更しないでください。",
  "connForm.restoreDir": "リストア先フォルダー（コンテナ内）",
  "connForm.restoreDirHint":
    "バックアップはこのフォルダー内の新しいサブフォルダーに展開され、既存のファイルは上書きされません。UID 999 に書き込み権限が必要です。空欄でリストアを無効にします。",
  "connForm.filesNote":
    "コンテナはマウントされたフォルダーのみ読み取れます。docker-compose にボリュームを追加してください。例: /var/www/app/uploads:/sources/uploads:ro",
  "connForm.testOkFiles": "バックアップ元フォルダーを読み取れました",
  "common.folder": "フォルダー",
  "restore.folder": "展開先: {dir}",
  "restore.targetFolder": "展開先フォルダー名（新規）",
  "settings.filesText": "バックアップ元 {addr} · 項目 <b>{db}</b>",
  "settings.restoreDir": "リストア先フォルダー <b>{user}</b>",
  // Google Drive web OAuth
  "googleOAuth.option": "Google OAuth でログイン",
  "googleOAuth.description":
    "Google にログインしてバックアップ先へのアクセスを許可します。トークンはサーバーに保存され、PC に rclone をインストールする必要はありません。",
  "googleOAuth.setup":
    "管理者はサーバーで GOOGLE_OAUTH_CLIENT_ID、GOOGLE_OAUTH_CLIENT_SECRET、GOOGLE_OAUTH_REDIRECT_URL を設定してください。README の Google OAuth 手順を参照してください。",
  "googleOAuth.callback": "Google に登録したコールバック URL:",
  "googleOAuth.tunnel":
    "SSH トンネルの場合、コールバック URL と同じ localhost とポートでアプリを開きます。サーバーの公開は不要です。",
  "googleOAuth.ready":
    "Google Drive を認証しました。接続を保持するには30分以内に保存してください。",
  "googleOAuth.connect": "Google Drive に接続",
  "googleOAuth.reconnect": "Google Drive に再接続",
  "googleOAuth.waiting": "Google の認証を待っています…",
  "googleOAuth.popup":
    "ブラウザーでログインのポップアップを許可して再試行してください。",
  "googleOAuth.required":
    "テストまたは保存の前に Google Drive に接続してください。",
  "googleOAuth.scope": "Drive のアクセス権",
  "googleOAuth.scopeFile": "このアプリで作成したファイルのみ（推奨）",
  "googleOAuth.scopeFull": "Drive 全体 — 既存フォルダーやバックアップ",
  "googleOAuth.scopeHint":
    "OAuth クライアントを変更して既存のバックアップにアクセスする場合は Drive 全体を選択して再接続します。Drive の全ファイルの読み取りと変更を許可します。",
};

export default ja;
