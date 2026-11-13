# DB Backup Manager

Ứng dụng tự triển khai để quản lý backup và khôi phục PostgreSQL, MySQL/MariaDB, MongoDB, SQL Server và thư mục file qua giao diện web. Quản lý nhiều kết nối, đặt lịch backup, lưu bản sao được mã hóa trên cloud và theo dõi kết quả trong cùng một nơi.

Backend viết bằng Go, frontend React + TypeScript, metadata lưu trong SQLite. Có thể chạy từ mã nguồn hoặc build thành một Docker image chứa ứng dụng và các công cụ backup cần thiết.

## Nội dung

- [Tính năng](#tính-năng)
- [Bắt đầu nhanh với Docker Compose](#bắt-đầu-nhanh-với-docker-compose)
- [Các loại database được hỗ trợ](#các-loại-database-được-hỗ-trợ)
- [Cấu hình và triển khai](#cấu-hình-và-triển-khai)
- [Người dùng và phân quyền](#người-dùng-và-phân-quyền)
- [Lưu trữ cloud và mã hóa](#lưu-trữ-cloud-và-mã-hóa)
- [Thông báo, kiểm thử restore và giám sát](#thông-báo-kiểm-thử-restore-và-giám-sát)
- [Quy trình vận hành](#quy-trình-vận-hành)
- [Phát triển và kiểm thử](#phát-triển-và-kiểm-thử)
- [Giới hạn](#giới-hạn)
- [Đóng góp](#đóng-góp)
- [Bảo mật và dữ liệu riêng](#bảo-mật-và-dữ-liệu-riêng)
- [Giấy phép](#giấy-phép)

## Tính năng

- Quản lý nhiều kết nối database và thư mục file trên giao diện web; kiểm tra kết nối trước khi lưu.
- Backup thủ công hoặc theo cron, quản lý retention, upload/download archive và kiểm tra SHA-256.
- Khôi phục vào database hoặc thư mục mới; cấu hình tài khoản restore riêng cho từng kết nối.
- Kiểm thử restore thủ công hoặc sau backup theo lịch, hiển thị kết quả cạnh bản backup.
- Lưu trữ offsite qua rclone: Google Drive, OneDrive, Dropbox, S3/R2/MinIO, B2, SFTP, WebDAV và các backend được ứng dụng hỗ trợ. Mã hóa bằng age trước khi upload, quản lý retention riêng trên cloud.
- Thông báo qua Telegram, Slack, Discord, SMTP hoặc webhook; cung cấp `/metrics` cho Prometheus.
- Tự thử lại backup/upload cloud bị lỗi và chạy bù lịch khi ứng dụng khởi động lại.
- Quản lý người dùng, phân quyền theo kết nối/database, nhật ký thao tác; tùy chỉnh tên, logo và múi giờ lịch chạy.
- HTTPS với chứng chỉ tự sinh hoặc chứng chỉ riêng. Container chạy bằng user không phải root, hỗ trợ root filesystem chỉ đọc và không cần Docker socket hay Portainer API.
- Giao diện tiếng Việt, tiếng Anh và tiếng Nhật.

## Bắt đầu nhanh với Docker Compose

### Yêu cầu

- Docker Engine hoặc Docker Desktop có Docker Compose v2.
- Một database có thể truy cập từ container, hoặc một thư mục được mount để backup file.
- Quyền ghi vào thư mục metadata và backup. Container chạy bằng UID/GID `999:999`.

Sau khi clone repository, chạy các lệnh sau từ thư mục gốc. Compose sẽ build image từ mã nguồn; không cần tài khoản registry.

```sh
cp .env.example .env
chmod 600 .env
docker network ls
```

Sửa `.env`, điền tối thiểu:

```dotenv
ADMIN_USER=admin
ADMIN_PASSWORD=REPLACE_WITH_A_STRONG_UNIQUE_PASSWORD
POSTGRES_NETWORK=your-existing-database-network
```

`ADMIN_PASSWORD` phải dài 12–72 byte. `POSTGRES_NETWORK` là tên network Docker thực tế mà container database đang dùng. Nếu chưa có network dùng chung, tạo bằng `docker network create db-backup`, đặt `POSTGRES_NETWORK=db-backup` và kết nối container database vào network đó. Dù biến có tên `POSTGRES_NETWORK`, network này có thể dùng cho các loại database khác.

Với đường dẫn mặc định `./data` và `./backups`, tạo thư mục có quyền phù hợp trên host Linux:

```sh
sudo install -d -o 999 -g 999 -m 700 ./data ./backups
docker compose up -d --build
docker compose ps
```

Mở **https://localhost:8088**, chấp nhận hoặc tin cậy chứng chỉ self-signed được tạo ở lần chạy đầu, rồi đăng nhập bằng tài khoản đã đặt trong `.env`. Nếu chạy trên máy chủ từ xa, xem [truy cập bằng SSH tunnel](#3-truy-cập-bằng-ssh-tunnel).

Trong **Databases → Thêm kết nối**, chọn loại database, nhập thông tin kết nối rồi bấm **Kiểm tra kết nối** và **Lưu**. Với DB trong cùng Docker network, dùng tên service/container làm host. Với thư mục file, xem [Files](#files-thư-mục-upload).

Chạy một bản backup thủ công và kiểm tra restore trước khi thiết lập lịch. Khi cần chẩn đoán lỗi khởi động:

```sh
docker compose logs --tail=100 db-backup-manager
```

Các biến tùy chọn có trong [.env.example](.env.example). Nếu triển khai bằng Portainer hoặc cần push image lên registry của bạn, xem [hướng dẫn deploy](deploy/README.md).

## Các loại database được hỗ trợ

Chọn **Loại database** khi thêm kết nối. Mỗi bản backup ghi nhớ loại DB của nó và chỉ restore được vào Server cùng loại.

| Loại | Cách backup | File tải về | Restore |
|---|---|---|---|
| PostgreSQL | `pg_dump --format=custom`, kiểm tra bằng `pg_restore --list` | `.dump` | `CREATE DATABASE` + `pg_restore --single-transaction --no-owner` |
| MySQL / MariaDB | `mariadb-dump --single-transaction --routines --triggers --events`, nén gzip, kiểm tra dòng `-- Dump completed` | `.sql.gz` | `CREATE DATABASE` + client `mariadb`; bỏ `DEFINER` để object thuộc tài khoản restore |
| MongoDB | `mongodump --db --archive --gzip`, kiểm tra header archive | `.archive.gz` | Từ chối nếu DB đích đã tồn tại, rồi `mongorestore --nsFrom/--nsTo` |
| SQL Server | `BACKUP DATABASE … WITH COPY_ONLY, CHECKSUM` (+ `COMPRESSION` nếu không phải Express) và `RESTORE VERIFYONLY` | `.bak` | `RESTORE DATABASE … WITH MOVE` sang file mới trong thư mục mặc định |
| Files (thư mục) | Nén cả thư mục mount vào container thành tar.gz, đọc lại toàn bộ archive để kiểm tra | `.tar.gz` | Giải nén vào một thư mục **mới** bên trong thư mục restore, không bao giờ ghi đè |

Upload chấp nhận đúng định dạng của Server được chọn (MySQL nhận cả `.sql` chưa nén). Mật khẩu không bao giờ nằm trên dòng lệnh: client tool đọc từ file tạm quyền `0600` trong `/tmp`, xóa ngay sau khi chạy.

### MySQL / MariaDB

Dùng client MariaDB 11.8 trong image, chạy được với MySQL 5.7/8.x và MariaDB 10.x/11.x. SSL mode `prefer` dùng TLS nếu server hỗ trợ (không kiểm tra chứng chỉ); `verify-full` kiểm tra chứng chỉ và hostname.

```sql
CREATE USER 'backup_user'@'%' IDENTIFIED BY 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
GRANT SELECT, SHOW VIEW, TRIGGER, EVENT, LOCK TABLES ON *.* TO 'backup_user'@'%';
-- Tùy chọn, cho restore: tạo DB mới và toàn quyền trên DB đó
CREATE USER 'restore_user'@'%' IDENTIFIED BY 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
GRANT ALL PRIVILEGES ON *.* TO 'restore_user'@'%';
```

Backup routine trên MySQL 8 cần thêm quyền `SHOW_ROUTINE` (hoặc `SELECT` toàn cục) nếu user không phải definer. Bảng không phải InnoDB (MyISAM) không nhất quán khi DB đang ghi vì `--single-transaction` chỉ áp dụng cho InnoDB.

### MongoDB

Trường **Database xác thực** là `authSource` (mặc định `admin`). Để trống tài khoản nếu MongoDB không bật xác thực. Kết nối tới một host/port; chưa hỗ trợ URI `mongodb+srv://` (Atlas) hay tùy chọn replica set riêng.

```js
db.getSiblingDB("admin").createUser({ user: "backup_user", pwd: "…", roles: ["backup"] })
// Tùy chọn, cho restore:
db.getSiblingDB("admin").createUser({ user: "restore_user", pwd: "…", roles: ["restore", "readAnyDatabase"] })
```

Backup một database không bao gồm user/role định nghĩa trong DB đó.

### SQL Server

SQL Server không có tool dump chạy từ xa: lệnh `BACKUP DATABASE` ghi file `.bak` **trên máy SQL Server**. Vì vậy mỗi kết nối SQL Server cần một **thư mục dùng chung** giữa SQL Server và container này, khai báo 2 đường dẫn:

- **Đường dẫn phía SQL Server**: thư mục như SQL Server thấy, ví dụ `/var/opt/mssql/backup` hoặc `D:\Backup`.
- **Đường dẫn trong container**: cùng thư mục đó được mount vào container manager, ví dụ `/mssql-backup`.

Manager ghi `.bak` vào đó, chuyển về `BACKUP_DIR`, rồi xóa file trong thư mục chung. Khi restore, manager chép bản backup vào thư mục chung (quyền `0644`) để SQL Server đọc, rồi xóa sau khi xong. Ví dụ cùng một Docker host:

```yaml
services:
  mssql:
    volumes:
      - mssql-backup:/var/opt/mssql/backup
  db-backup-manager:
    group_add:
      - "10001" # group mssql: SQL Server tạo file .bak quyền 0640
    volumes:
      - mssql-backup:/mssql-backup
volumes:
  mssql-backup:
```

Yêu cầu quyền (đã kiểm thử với SQL Server 2022 trên Linux):

- SQL Server (user `mssql`, UID/GID 10001) tạo `.bak` với quyền `0640`, nên container manager (UID 999) phải thuộc group 10001 (`group_add`) mới đọc được.
- Thư mục phải ghi được bởi cả hai phía, ví dụ `chmod 0777` hoặc `2775` với group 10001. **Không dùng sticky bit** (`1777`): manager sẽ không xóa được file do SQL Server tạo.
- Khác máy thì dùng share SMB/NFS mount ở cả hai phía, ánh xạ quyền tương đương. **Kiểm tra kết nối** xác minh đăng nhập và quyền ghi thư mục trong container, nhưng chỉ lần backup đầu tiên mới xác nhận hai đường dẫn trỏ cùng một chỗ.

```sql
CREATE LOGIN backup_user WITH PASSWORD = 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
-- Trong từng database cần backup:
CREATE USER backup_user FOR LOGIN backup_user;
ALTER ROLE db_backupoperator ADD MEMBER backup_user;
-- Để liệt kê dung lượng DB (tùy chọn):
GRANT VIEW ANY DEFINITION TO backup_user;
-- Tùy chọn, cho restore:
CREATE LOGIN restore_user WITH PASSWORD = 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
ALTER SERVER ROLE dbcreator ADD MEMBER restore_user;
```

Backup dùng `COPY_ONLY` nên không làm lệch chuỗi backup differential/log hiện có. `RESTORE VERIFYONLY` chỉ chạy khi tài khoản backup có quyền CREATE DATABASE; nếu không, dựa vào `BACKUP … WITH CHECKSUM`. Database được restore giữ nguyên user/owner bên trong bản backup; cấp quyền cho ứng dụng sau khi kiểm tra.

### Files (thư mục upload)

Backup một thư mục file, ví dụ thư mục upload của web app, cùng lịch, retention, cloud, thông báo và kiểm thử restore như database. Mỗi kết nối **Files** là **một thư mục**. Muốn backup nhiều thư mục thì tạo nhiều kết nối.

Container chỉ đọc được thư mục đã mount vào nó. Thêm volume vào `docker-compose.yml`:

```yaml
    volumes:
      - /var/www/app/uploads:/sources/uploads:ro   # thư mục nguồn, chỉ đọc
      - ./restore:/restore                         # thư mục restore (tùy chọn)
```

Rồi thêm kết nối loại **Files**: **Thư mục nguồn** `/sources/uploads`, **Thư mục restore** `/restore` (để trống nếu không cần restore).

- Container chạy với UID 999: file nguồn phải đọc được bởi UID này (thường là quyền `644`/`755` mặc định). Thư mục restore phải ghi được bởi UID 999, ví dụ `chown 999:999 ./restore`.
- Backup giữ cấu trúc thư mục, quyền (bỏ setuid/setgid), thời gian sửa và symlink. Không giữ chủ sở hữu (UID/GID). Socket, device, pipe bị bỏ qua.
- Restore luôn tạo thư mục con mới `/restore/<tên đích>` và từ chối nếu đã tồn tại. Kiểm tra xong thì tự chép về chỗ cũ, ví dụ `rsync -a /path/restore/<tên>/ /var/www/app/uploads/`.
- Archive upload hoặc kéo từ cloud được kiểm tra trước: từ chối đường dẫn tuyệt đối, `..`, hardlink, device. Symlink được tạo sau cùng nên không file nào bị ghi xuyên qua symlink.
- Mỗi lần backup nén lại **toàn bộ** thư mục (không tăng dần). Phù hợp thư mục vài chục GB trở xuống. Thư mục rất lớn có thể cần tăng `JOB_TIMEOUT`.
- File đang được ghi trong lúc backup có thể chỉ được lưu một phần. File bị thu nhỏ trong lúc đọc làm backup lỗi và được thử lại. Nên đặt lịch backup file ngay sau lịch backup database để hai bản gần thời điểm nhau.
- Thư mục nguồn không được chứa (hoặc nằm trong) `BACKUP_DIR`, và không được trùng với thư mục restore.

## Cấu hình và triển khai

### 1. Chuẩn bị PostgreSQL

Xác định tên Docker network và DNS alias/service của PostgreSQL hiện tại. Không mở port PostgreSQL ra Internet. Manager phải truy cập được host/port của từng kết nối; với PostgreSQL trong Docker, dùng cùng external network và DNS alias của container.

Tạo user backup bằng tài khoản quản trị PostgreSQL. Ví dụ (thay mật khẩu trước khi chạy):

```sql
CREATE ROLE backup_user LOGIN PASSWORD 'REPLACE_WITH_STRONG_BACKUP_PASSWORD';
GRANT pg_read_all_data TO backup_user;
GRANT CONNECT ON DATABASE app_db TO backup_user;
```

`pg_read_all_data` không bỏ qua Row Level Security. Nếu DB có RLS, large objects, extension hoặc ACL đặc biệt, DBA cần cấp quyền phù hợp và kiểm thử backup từng database. Không cấp superuser mặc định. Để tính dung lượng tất cả DB, user phải có quyền CONNECT tới từng DB cần quản lý (hoặc quyền giám sát phù hợp).

Tài khoản restore tùy chọn, tách khỏi tài khoản backup:

```sql
CREATE ROLE restore_user LOGIN CREATEDB PASSWORD 'REPLACE_WITH_DIFFERENT_RESTORE_PASSWORD';
GRANT CONNECT ON DATABASE postgres TO restore_user;
```

Object được restore sẽ thuộc tài khoản restore (`--no-owner --no-privileges`). Extension cần superuser hoặc cấu hình đặc biệt có thể khiến restore thất bại. Cấp quyền cho ứng dụng sau khi DBA xác minh dữ liệu đích.

### 2. Cấu hình và build

Trên máy chạy ứng dụng, trong thư mục dự án:

```bash
cp .env.example .env
chmod 600 .env
```

Cấu hình `ADMIN_PASSWORD` và `POSTGRES_NETWORK` trong `.env`, sau đó đăng nhập và vào **Databases → Thêm kết nối**. Nhập tên gợi nhớ, host, port, tài khoản, mật khẩu, database kết nối (mặc định `postgres`) và SSL mode. Bấm **Kiểm tra kết nối**, rồi **Lưu kết nối**. Có thể lưu nhiều máy chủ và chọn từng kết nối để xem DB; mỗi backup hoặc lịch tự động gắn với kết nối được chọn. “Tất cả database” áp dụng cho máy chủ đó.

Host được truy cập từ máy chủ/container chạy manager. `localhost` trong container không trỏ tới máy cá nhân. Kết nối được lưu trong SQLite và mật khẩu được mã hóa bằng khóa `connections.key` cạnh file metadata. Cần sao lưu **cả metadata và file khóa** cùng nhau để khôi phục cấu hình. Không xóa file khóa khi đang có kết nối đã lưu. Viewer chỉ xem; admin quản lý kết nối. Kết nối có job đang chạy/chờ không thể sửa/xóa; cần xóa các lịch liên quan trước khi xóa kết nối. Archive và lịch sử được giữ lại khi xóa kết nối.

Kết nối tạo trên UI dùng cho liệt kê DB, backup, lịch tự động và restore. Trong **Databases → Thêm/Sửa kết nối**, nhập **Tài khoản restore** và **Mật khẩu restore** riêng cho Server đó; để trống tài khoản để tắt restore. Tài khoản cần LOGIN, CREATEDB và quyền CONNECT vào database maintenance. **Kiểm tra kết nối** xác minh cả kết nối backup và quyền CREATEDB của tài khoản restore đã nhập. Việc kiểm tra không đảm bảo mọi extension trong archive đều có đủ quyền để restore.

Trong **Bản backup**, lọc theo Server rồi database; upload gắn archive với Server được chọn. Bấm restore trên archive, chọn Server đích đã cấu hình restore (có thể khác Server nguồn), nhập tên DB mới, nhập lại để xác nhận và xác nhận tin cậy archive. Không ghi đè DB hiện có. Server đã xóa vẫn có archive để download hoặc restore sang Server khác. Mật khẩu backup và restore được mã hóa; để trống mật khẩu khi sửa để giữ giá trị đã lưu, hoặc chọn xóa mật khẩu tương ứng.

Mọi cấu hình kết nối DB (host, tài khoản, mật khẩu, restore, SSL) chỉ được quản lý trên web; `.env` không còn chứa thông tin DB. Các biến `PGHOST`, `PGUSER`, `PGPASSWORD`, `RESTORE_PGUSER`… nếu còn đặt sẽ bị bỏ qua.

Các biến `.env`:

| Biến | Ý nghĩa |
| --- | --- |
| `ADMIN_USER`, `ADMIN_PASSWORD` | Tài khoản admin. `ADMIN_PASSWORD` bắt buộc (12–72 byte, dùng mật khẩu mạnh) và là mật khẩu admin **cho tới khi** đổi trên web; xem [Mật khẩu admin](#mật-khẩu-admin) |
| `VIEWER_PASSWORD` | Tùy chọn; tài khoản `viewer` chỉ xem (toàn bộ kết nối, không download); mật khẩu khác admin |
| `POSTGRES_NETWORK` | Bắt buộc nhập tên external Docker network hiện có (`docker network ls`); không có tên mặc định |
| `PG_MAJOR` | Version client trong image, mặc định `18`; phải >= major server |
| `DATA_DIR`, `BACKUP_DIR` | Thư mục host cho metadata và archive; khuyên dùng đường dẫn tuyệt đối trên VM |
| `TZ` | Múi giờ mặc định của lịch chạy (`Asia/Ho_Chi_Minh`). Admin có thể đổi trong UI (menu tài khoản góc trên phải → Đổi múi giờ lịch chạy); giá trị trong UI được ưu tiên |
| `JOB_TIMEOUT` | Timeout mỗi job, mặc định `2h` |
| `MAX_UPLOAD_BYTES` | Mặc định `10737418240` (10 GiB) |
| `RCLONE_BINARY` | Tùy chọn, đường dẫn rclone; mặc định `rclone` trong `PATH` (image đã có sẵn) |

Image chạy bằng user `postgres` (UID/GID 999:999). Tạo thư mục mới và cấp quyền tương ứng, ví dụ khi dùng đường dẫn dưới đây (đồng thời sửa hai biến trong `.env`):

```bash
sudo install -d -o 999 -g 999 -m 700 /opt/db-backup-manager/data /opt/backups/db
```

Không cấp quyền truy cập thư mục archive cho user không cần thiết. Nếu dùng đường dẫn mặc định `./data` và `./backups`, cũng tạo/cấp quyền các thư mục này trước khi chạy. Không dùng `chmod 777`.

```bash
docker compose build
docker compose up -d
docker compose ps
docker compose logs --tail=100 db-backup-manager
```

Compose chỉ publish `127.0.0.1:8088:8080`, root filesystem read-only, bỏ Linux capabilities, không mount Docker socket. Healthcheck chỉ kiểm tra backend/SQLite; trạng thái PostgreSQL hiển thị riêng trên dashboard.

### 3. Truy cập bằng SSH tunnel

Trên máy cá nhân:

```bash
ssh -N -L 8088:127.0.0.1:8088 ubuntu@SERVER_IP
```

Mở **https://localhost:8088**, đăng nhập bằng tài khoản đã đặt trong `.env`.

### HTTPS

Image Docker mặc định chạy HTTPS (`TLS_MODE=self-signed`). Lần khởi động đầu, server tự sinh chứng chỉ self-signed (ECDSA P-256, hạn 10 năm) và lưu trong `DATA_DIR/tls/` (`cert.pem`, `key.pem`), nên deploy lại không đổi chứng chỉ. Mở bằng `http://` sẽ tự chuyển sang `https://` (cùng port). Trình duyệt sẽ cảnh báo lần đầu vì chứng chỉ không do CA ký: chọn **Advanced → Proceed** (hoặc import `cert.pem` vào máy để hết cảnh báo).

Chứng chỉ luôn hợp lệ cho `localhost`, `127.0.0.1` và hostname container. Nếu truy cập bằng IP/tên miền của server, thêm vào `TLS_HOSTS` (phân cách bằng dấu phẩy), ví dụ `TLS_HOSTS=192.0.2.10,backup.example.com`. Khi `TLS_HOSTS` đổi, hoặc chứng chỉ còn dưới 30 ngày, server tự sinh lại khi khởi động.

| `TLS_MODE` | Ý nghĩa |
|---|---|
| `self-signed` | Mặc định trong Docker. Tự sinh và giữ chứng chỉ. |
| `file` | Dùng chứng chỉ riêng: mount file vào container và đặt `TLS_CERT_FILE`, `TLS_KEY_FILE`. |
| `off` | HTTP thường, ví dụ khi đặt sau reverse proxy HTTPS (khi đó đặt thêm `COOKIE_SECURE=true`). |

Cookie đăng nhập tự bật `Secure` khi TLS bật; `COOKIE_SECURE` chỉ cần khi muốn ghi đè. Chạy local bằng `make run` vẫn là HTTP nếu không đặt `TLS_MODE`.

### 4. Portainer

Xem hướng dẫn đầy đủ tại [deploy/README.md](deploy/README.md): truyền repository của bạn qua `IMAGE` khi push/deploy image, tạo stack từ [deploy/docker-compose.portainer.yml](deploy/docker-compose.portainer.yml), dùng named volume nên không cần tạo thư mục/`chown` thủ công.

## Người dùng và phân quyền

Admin trong `.env` (`ADMIN_USER`) luôn có toàn quyền. Các tài khoản khác tạo trong UI: menu người dùng góc phải → **Quản lý người dùng** (`#/users`).

- **Quản trị viên**: toàn quyền như admin `.env` — kết nối, đích cloud, người dùng, mọi database.
- **Người dùng**: chỉ thấy kết nối, database, backup, lịch và job trong phạm vi được cấp. Mỗi dòng quyền gồm *Kết nối* (hoặc tất cả) × *Database* (để trống = tất cả) × *Mức*:
  - **Xem**: xem backup, lịch, job.
  - **Vận hành**: thêm backup ngay, download, upload, quản lý lịch, xóa backup, đẩy lên cloud.
  - Backup/lịch cho "Tất cả database" và restore (vào kết nối đích) cần quyền vận hành trên **toàn bộ** database của kết nối đó.
- Quản lý kết nối, đích cloud và kéo file từ cloud chỉ dành cho quản trị viên; duyệt file trên cloud cần quyền xem toàn bộ kết nối.
- Người dùng tự đổi mật khẩu trong **Hồ sơ & mật khẩu**. Admin `.env` cũng đổi được ở đây; mật khẩu mới được lưu trong metadata và giữ nguyên kể cả khi sửa `ADMIN_PASSWORD` hay khởi động lại.

### Mật khẩu admin

`ADMIN_PASSWORD` trong `.env` là mật khẩu mặc định của admin, không chỉ có tác dụng ở lần chạy đầu:

| Tình huống | Mật khẩu admin dùng để đăng nhập |
| --- | --- |
| Chưa từng đổi mật khẩu trên web | `ADMIN_PASSWORD` trong `.env`. Sửa `.env` rồi chạy `docker compose up -d` là đổi được |
| Đã đổi trong **Hồ sơ & mật khẩu** | Mật khẩu mới (lưu trong metadata). `ADMIN_PASSWORD` bị bỏ qua, kể cả khi sửa `.env` hay khởi động lại |
| Chạy `reset-admin-password` | Quay lại dùng `ADMIN_PASSWORD` trong `.env` |

Dù đã bị bỏ qua, `ADMIN_PASSWORD` vẫn **bắt buộc** phải có (12–72 byte), nếu không server không khởi động. Mật khẩu đổi trên web nằm trong `data/metadata.db`: khôi phục metadata từ bản sao lưu cũng khôi phục mật khẩu admin tại thời điểm đó.

### Quên mật khẩu admin

Lệnh reset bỏ mật khẩu đã đặt trên web để `ADMIN_PASSWORD` trong `.env` có hiệu lực trở lại. Không cần khởi động lại; các phiên admin đang mở bị đăng xuất.

```bash
docker compose exec db-backup-manager db-backup-manager reset-admin-password
```

Nếu cũng không nhớ `ADMIN_PASSWORD`: sửa nó trong `.env`, chạy `docker compose up -d` để container nhận giá trị mới, rồi chạy lệnh reset ở trên.

Chạy ngoài Docker: `METADATA_DB=data/metadata.db ./bin/db-backup-manager reset-admin-password`. Portainer: mở Console của container và chạy `db-backup-manager reset-admin-password`.


## Lưu trữ cloud và mã hóa

Image Docker có sẵn [rclone](https://rclone.org), được build từ phiên bản mã nguồn cố định và xác minh dependency qua Go checksum database. Khi chạy local cần cài rclone trong `PATH` hoặc đặt `RCLONE_BINARY`.

### Luồng xử lý

1. Backup hoàn tất (thủ công hoặc theo lịch có chọn đích cloud) → tạo job **Upload lên cloud**.
2. Job kiểm tra SHA-256 archive, mã hóa bằng [age](https://age-encryption.org) vào file tạm trong `BACKUP_DIR`, upload bằng `rclone copyto`, đối chiếu dung lượng trên cloud rồi xóa file tạm.
3. File trên cloud: `<thư mục>/<tên kết nối>/<database>/<database>_<YYYYMMDDTHHMMSSZ>_<id>.dump.age`.
4. Retention trên cloud (theo từng đích, `0` = giữ vĩnh viễn) chạy cùng job dọn dẹp, **luôn giữ bản mới nhất** của mỗi database. Retention cục bộ không ảnh hưởng bản trên cloud.

### Thêm đích cloud

Vào **Lưu trữ cloud → Thêm đích cloud**:

- **Google Drive — OAuth trên web**: chọn **Google Drive → Đăng nhập bằng Google OAuth → Kết nối Google Drive**, đăng nhập/cấp quyền rồi bấm **Lưu**. Máy cá nhân không cần cài rclone. Quản trị viên cấu hình OAuth Client một lần theo hướng dẫn bên dưới. Sửa đích cloud và bấm **Kết nối lại Google Drive** khi token không còn hợp lệ.
- **Cấu hình rclone thủ công — Google Drive / OneDrive / Dropbox** (OAuth): trên máy có trình duyệt, chạy `rclone config` → `n` → chọn `drive` / `onedrive` / `dropbox`, đăng nhập. Với Google Drive nên chọn scope `drive.file`. Sau đó chạy `rclone config show <tên-remote>` và dán kết quả vào ô **Cấu hình rclone**. Token được tự gia hạn; token mới được mã hóa và lưu lại sau mỗi lần chạy.
- **S3 / R2 / MinIO / Wasabi**: dán trực tiếp, ví dụ:

  ```ini
  type = s3
  provider = AWS
  access_key_id = AKIA...
  secret_access_key = ...
  region = ap-southeast-1
  ```

  Thư mục đích bắt đầu bằng tên bucket, ví dụ `my-bucket/db-backups`. Dùng access key chỉ có quyền trên bucket backup.

Cấu hình rclone (kể cả token) được mã hóa AES-GCM bằng `connections.key`, không bao giờ trả về trình duyệt. Không chấp nhận remote kiểu `local`/`alias`/`crypt`… và tùy chọn `ssh`. Bấm **Kiểm tra** để ghi, đọc và xóa một file thử trên cloud.

### Google Drive OAuth trên web, qua SSH tunnel

Ứng dụng dùng [OAuth 2.0 Web Server của Google](https://developers.google.com/identity/protocols/oauth2/web-server). Rclone vẫn chạy ở máy chủ để truyền file và tự gia hạn access token. Máy chủ không cần public Internet; máy chủ và trình duyệt cần truy cập Google qua Internet chiều ra.

1. Trong Google Cloud Console, bật **Google Drive API** cho project. Cấu hình Google Auth Platform (Audience / Data Access), rồi tạo OAuth Client loại **Web application**.
2. Đăng ký **Authorized redirect URI** đúng tuyệt đối với `GOOGLE_OAUTH_REDIRECT_URL`, bao gồm scheme, hostname, port và đường dẫn `/api/destinations/google/callback`. Không dùng IP LAN làm callback; dùng `localhost` qua SSH tunnel. `localhost` và `127.0.0.1` là hai hostname khác nhau, phải khớp URL đã đăng ký.
3. Thêm ba biến vào `.env` (Docker Compose đã truyền chúng vào container), ví dụ với Docker mặc định dùng HTTPS ở cổng host 8088:

   ```dotenv
   GOOGLE_OAUTH_CLIENT_ID=your-client-id.apps.googleusercontent.com
   GOOGLE_OAUTH_CLIENT_SECRET=your-client-secret
   GOOGLE_OAUTH_REDIRECT_URL=https://localhost:8088/api/destinations/google/callback
   ```

   Giữ `.env` riêng tư và không commit Client Secret. Khởi động lại/recreate container sau khi đổi cấu hình. Nếu chạy binary trực tiếp, truyền các biến này qua môi trường của process.

4. Mở tunnel từ máy cá nhân:

   ```bash
   ssh -N -L 8088:127.0.0.1:8088 user@server
   ```

   Mở **https://localhost:8088** trên trình duyệt, chấp nhận/tin cậy certificate nội bộ nếu dùng self-signed, đăng nhập ứng dụng rồi kết nối Google Drive. Cổng phía máy chủ trong ví dụ là cổng Docker đã publish (`127.0.0.1:8088` → container `8080`). Nếu port local bị chiếm, đổi port local, URL callback và đăng ký Google cho khớp.

   Nếu ứng dụng chạy HTTP: đặt `TLS_MODE=off`, `COOKIE_SECURE=false` và callback `http://localhost:8088/api/destinations/google/callback`. Nếu binary nghe ở port khác, đổi đích tunnel tương ứng. Không cần thêm tunnel cho port rclone 53682.

5. Trong form chọn phạm vi quyền:
   - **Chỉ file do ứng dụng này tạo** (`drive.file`, mặc định): giới hạn quyền vào file mà OAuth Client này được phép truy cập. Thích hợp cho backup mới.
   - **Toàn bộ Drive** (`drive`): dùng khi cần thư mục hoặc backup cũ tạo bởi OAuth Client khác (ví dụ trước đây dùng Client mặc định của rclone). Quyền này cho phép đọc/sửa mọi file Drive; Google có thể yêu cầu xác minh ứng dụng tùy phạm vi triển khai. Đổi Client rồi dùng `drive.file` không tự cấp quyền vào file do Client cũ tạo.
6. Sau khi Google báo kết nối thành công, đóng cửa sổ Google, quay lại form, bấm **Kiểm tra** nếu cần rồi **Lưu**. Quyền mới chưa thay thế cấu hình đã lưu cho đến khi bấm Lưu. Khi kết nối lại đích Drive cũ, ứng dụng giữ `root_folder_id` và `team_drive` đã cấu hình. Chọn đúng thư mục cũ (hoặc Folder ID qua cấu hình rclone thủ công) khi cần duyệt backup hiện có.

Phiên cấp quyền chờ tối đa 10 phút; kết quả chờ lưu tối đa 30 phút, gắn với phiên đăng nhập admin và mất khi restart server. Kiểm tra đích cloud không tiêu thụ kết quả OAuth; lưu thành công tiêu thụ nó. Token và Client Secret không trả về trình duyệt; cấu hình Drive được mã hóa bằng `connections.key` như cấu hình rclone thủ công. Giữ Client ID/Secret đi kèm token trong cấu hình giúp rclone tiếp tục gia hạn sau khi server khởi động lại.

Nếu Audience là **External** và Publishing status còn **Testing**, refresh token cho Drive hết hạn sau **7 ngày**, kể cả backup chạy thường xuyên. Chuyển sang **In production** và kết nối lại để tránh giới hạn của Testing; trạng thái này không yêu cầu public máy chủ backup. Token vẫn có thể bị thu hồi hoặc hết hiệu lực theo chính sách Google/Workspace. Khi gặp `invalid_grant`, mở đích cloud và kết nối lại. Việc thêm OAuth trên web giúp đăng nhập lại thuận tiện, không loại bỏ chính sách hết hạn của Google.

### Phương thức mã hóa

| | Public key (khuyến nghị) | Passphrase |
| --- | --- | --- |
| Thuật toán | age X25519 + ChaCha20-Poly1305 | age scrypt + ChaCha20-Poly1305 |
| Máy chủ lưu | Chỉ public key | Passphrase (mã hóa bằng `connections.key`) |
| Lộ máy chủ/cloud | Backup vẫn **không** giải mã được | Kẻ tấn công chiếm máy chủ giải mã được |
| Khôi phục cần | Private key (`AGE-SECRET-KEY-1…`) | Passphrase |

- Bấm **Tạo cặp khóa mới** để sinh khóa: private key chỉ hiển thị **một lần** (sao chép hoặc tải `.txt`), máy chủ không lưu. Có thể tự tạo bằng `age-keygen -o key.txt` và chỉ dán public key. Có thể nhập nhiều public key (mỗi dòng một khóa) để có khóa dự phòng.
- Cất private key/passphrase **tách khỏi máy chủ và tài khoản cloud** (password manager, két…). Mất khóa = mất backup.
- Đổi khóa chỉ áp dụng cho lần upload sau; file cũ vẫn cần khóa cũ.
- age xác thực từng khối dữ liệu: file bị sửa trên cloud sẽ giải mã thất bại thay vì cho ra dữ liệu sai.

### Khôi phục từ cloud

- **Trên giao diện**: **Lưu trữ cloud → Duyệt file & tải về**, chọn file, chọn kết nối gắn kèm, nhập private key (chỉ giữ trong RAM tới khi job xong). Bản giải mã xuất hiện trong danh sách backup với nguồn “Từ cloud”, rồi restore như bình thường. Dùng được cả trên máy chủ mới (chỉ cần thêm lại đích cloud).
- **Thủ công** (khi mất máy chủ):

  ```bash
  rclone copy gdrive:db-backups/Production/app_db/app_db_20261102T020000Z_1a2b3c4d.dump.age .
  age -d -i key.txt app_db_20261102T020000Z_1a2b3c4d.dump.age > app_db.dump
  # passphrase: age -d app_db_...dump.age > app_db.dump
  pg_restore --list app_db.dump
  createdb app_db_restored && pg_restore --no-owner -d app_db_restored app_db.dump
  ```

## Thông báo, kiểm thử restore và giám sát

### Thông báo

Admin vào **Thông báo** (menu Tài khoản) → **Thêm kênh**, chọn loại, điền cấu hình rồi **Gửi thử** trước khi lưu. Mỗi kênh chọn sự kiện và ngôn ngữ tin nhắn (Tiếng Việt, English, 日本語) riêng:

| Sự kiện | Khi nào gửi |
|---|---|
| Job thất bại | Backup, backup theo lịch, restore, kiểm thử restore, upload/tải cloud hoặc dọn bản cũ lỗi. Backup và upload cloud chỉ báo sau lần thử lại cuối. |
| Lịch không chạy | Một lịch đang bật đã quá giờ chạy 15 phút mà không có job nào được tạo (scheduler bị kẹt, container dừng…). Mỗi lượt bị lỡ chỉ báo một lần; job lỗi thì đã có thông báo "thất bại" riêng. |
| Lâu không có backup | Lịch đang bật nhưng quá `BACKUP_MAX_AGE` (mặc định `24h`; hoặc 1 chu kỳ lịch + `JOB_TIMEOUT` nếu dài hơn, ví dụ lịch hằng ngày ~26h, hằng tuần ~7 ngày) chưa có bản backup thành công. Bắt được cả trường hợp job vẫn chạy nhưng lỗi liên tục. Báo một lần cho tới khi có backup thành công. |
| Backup nhỏ bất thường | Bản backup mới nhỏ hơn bản trước của cùng database quá `BACKUP_SHRINK_PERCENT` (mặc định `50`%). Bỏ qua khi bản trước dưới 1 MiB. |
| Mất kết nối | Cứ 5 phút kiểm tra mọi kết nối (giống nút *Kiểm tra kết nối*). Báo khi lỗi 2 lần liên tiếp (~10 phút), và báo lại một lần khi kết nối khôi phục. Webhook nhận `event` là `connection_down` / `connection_up`. |
| Cloud không truy cập được | Cứ 30 phút ghi/đọc thử một file nhỏ lên mỗi nơi lưu cloud. Báo khi lỗi 2 lần liên tiếp và khi hoạt động lại (`destination_down` / `destination_up`). |
| Ổ đĩa sắp đầy | Ổ chứa `BACKUP_DIR` còn trống dưới `DISK_ALERT_PERCENT` (mặc định `10`%), kiểm tra 10 phút/lần; báo lại khi trống trên ngưỡng + 2% (`disk_low` / `disk_ok`). |
| Có yêu cầu restore | Ngay khi có người yêu cầu restore, kèm người thực hiện, bản backup và database đích. |
| Đăng nhập sai nhiều lần | Một địa chỉ IP đăng nhập sai 10 lần và bị khóa 15 phút. Sau reverse proxy, mọi request có thể mang cùng IP của proxy. |
| Job hoàn tất | Backup, restore và kiểm thử restore thành công. Có thể khá nhiều tin. |

| Loại | Cấu hình |
|---|---|
| Telegram | Bot token từ @BotFather và Chat ID (nhóm có dạng `-100…`) |
| Slack / Discord | Incoming webhook URL (`https://…`) |
| Webhook | URL nhận POST JSON (`event`, `title`, `text`, `connection`, `database`, `job`…). Secret tùy chọn: header `X-DBBM-Signature: sha256=<HMAC-SHA256 của body>` |
| Email | Máy chủ SMTP, STARTTLS (587) / SSL (465) / không mã hóa, tài khoản tùy chọn, người gửi, tối đa 20 người nhận |

Token, URL webhook và mật khẩu SMTP được mã hóa AES-GCM như mật khẩu kết nối, không bao giờ trả về API; để trống khi sửa là giữ nguyên. Gửi lỗi được ghi vào nhật ký thao tác (`notifier.failed`) và metric `dbbm_notification_failures_total`.

### Thử lại và chạy bù

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `JOB_RETRIES` | `2` | Số lần thử lại backup / upload cloud lỗi (0–10). Trong lúc chờ, job ở trạng thái "Đang chờ" kèm lỗi lần trước. |
| `JOB_RETRY_DELAY` | `2m` | Thời gian chờ trước lần thử lại đầu tiên, gấp đôi sau mỗi lần. |
| `DISK_ALERT_PERCENT` | `10` | Báo "Ổ đĩa sắp đầy" khi dung lượng trống dưới mức này (%). `0` = tắt. |
| `BACKUP_SHRINK_PERCENT` | `50` | Báo "Backup nhỏ bất thường" khi bản mới nhỏ hơn bản trước quá mức này (%). `0` = tắt. |
| `BACKUP_MAX_AGE` | `24h` | Báo "Lâu không có backup" khi quá thời gian này chưa có backup thành công. `0` = tắt. |
| `CATCH_UP_MISSED` | `true` | Lúc khởi động, lịch nào đã lỡ giờ chạy trong khi dừng sẽ chạy một lần (lỡ nhiều lượt vẫn chỉ một bản backup). |

Restart trong lúc đang chờ thử lại sẽ đánh dấu job thất bại như các job đang chờ khác.

### Kiểm thử restore

Kiểm thử restore chứng minh bản backup **restore được**, điều mà checksum và `pg_restore --list` không làm được. Trình tự: kiểm tra checksum → restore vào database mới tên `dbbm_verify_<ngẫu nhiên>` bằng tài khoản restore → mở database và đếm số bảng/collection → **xóa database tạm** (kể cả khi restore lỗi hoặc hết `JOB_TIMEOUT`). Manager kiểm tra tên tạm chưa tồn tại trước khi tạo, nên không bao giờ xóa database có sẵn.

- **Chạy tay**: nút bình thí nghiệm ở mỗi bản backup, chọn Server kiểm thử (cùng loại DB, đã bật restore).
- **Tự động**: khi tạo lịch, bật *Tự kiểm thử restore sau mỗi lần backup* và chọn Server kiểm thử.
- Kết quả hiện cạnh bản backup (*Restore OK* / *Restore lỗi*, rê chuột để xem chi tiết) và được gửi qua kênh thông báo.

Kiểm thử dùng CPU, RAM, đĩa của Server kiểm thử và cần dung lượng trống bằng database được restore. Với database lớn, nên trỏ sang một Server staging thay vì production. Quyền cần thêm cho tài khoản restore: PostgreSQL xóa được DB do mình tạo (owner); MySQL quyền `DROP`; MongoDB vai trò `restore` (nếu thiếu `dropDatabase`, manager xóa từng collection); SQL Server `dbcreator`. Với SQL Server, nếu tài khoản restore không đọc được DB vừa restore, kết quả chỉ xác nhận DB ở trạng thái ONLINE. Nếu xóa database tạm thất bại, job báo lỗi kèm tên DB để xóa thủ công.

### Prometheus

Đặt `METRICS_TOKEN` (tối thiểu 16 ký tự) để bật `GET /metrics`; không đặt thì endpoint trả 404. Cấu hình scrape:

```yaml
scrape_configs:
  - job_name: db-backup-manager
    authorization:
      credentials: "<METRICS_TOKEN>"
    static_configs:
      - targets: ["db-backup-manager:8080"]
```

| Metric | Nhãn | Ý nghĩa |
|---|---|---|
| `dbbm_backup_last_success_timestamp_seconds` | connection, database | Lúc job backup thành công gần nhất kết thúc |
| `dbbm_backup_last_failure_timestamp_seconds` | connection, database | Lúc job backup lỗi gần nhất kết thúc |
| `dbbm_backup_last_duration_seconds` | connection, database | Thời gian chạy của backup thành công gần nhất |
| `dbbm_backup_last_timestamp_seconds`, `dbbm_backup_last_size_bytes`, `dbbm_backups`, `dbbm_backups_bytes` | connection, database | Bản backup đang lưu |
| `dbbm_verify_last_timestamp_seconds`, `dbbm_verify_last_success` | connection, database | Kết quả kiểm thử restore gần nhất |
| `dbbm_schedule_stale` | connection, database | 1 khi lịch đã lỡ giờ mà không tạo job |
| `dbbm_destination_up` | destination | 1 khi lần kiểm tra nơi lưu cloud gần nhất (30 phút/lần) thành công, 0 khi lỗi |
| `dbbm_connection_up` | connection | 1 khi lần kiểm tra kết nối gần nhất (5 phút/lần) thành công, 0 khi lỗi |
| `dbbm_jobs` | kind, status | Số job trong lịch sử; `dbbm_queue_length` job đang chờ |
| `dbbm_offsite_last_upload_timestamp_seconds` | destination | Lần upload cloud gần nhất |
| `dbbm_backup_disk_free_bytes`, `dbbm_backup_disk_size_bytes` | | Dung lượng đĩa của `BACKUP_DIR` |
| `dbbm_notification_failures_total` | | Số thông báo gửi lỗi từ lúc khởi động |

Ví dụ cảnh báo:

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

## Quy trình vận hành

1. Kiểm tra dashboard đã Connected và hiện đúng DB cần backup.
2. Chạy backup thủ công, xem job trong Lịch sử; chỉ job `completed` mới tạo backup để download/restore.
3. Tạo lịch, ví dụ `0 2 * * *` (hàng ngày 02:00), retention 14 ngày.
4. Thêm ít nhất một kênh thông báo và bật kiểm thử restore tự động cho lịch quan trọng. Kiểm thử tự động xác nhận bản backup restore được; vẫn nên định kỳ restore một archive vào DB mới và kiểm tra dữ liệu nghiệp vụ từ ứng dụng (row, extension, quyền).
5. Bật đích cloud cho lịch backup (xem phần Lưu trữ cloud); định kỳ thử tải về và giải mã một bản từ cloud. Metadata (`DATA_DIR`) không được upload tự động: sao lưu riêng. Để copy SQLite an toàn, dừng Manager trước khi copy toàn bộ `DATA_DIR`, hoặc dùng SQLite online backup API; không chỉ copy `metadata.db` khi WAL đang hoạt động.
6. Archive chỉ được hiển thị khi có metadata. Không tự ý thêm/đổi tên file dưới `BACKUP_DIR`; dùng Upload để nhập archive. Crash có thể để lại `.partial` hoặc archive chưa có metadata; chỉ dọn file mồ côi khi Manager đã dừng và đã kiểm tra metadata.

Restore thất bại sau bước tạo DB sẽ giữ database đích để DBA kiểm tra. Không tự drop DB (riêng kiểm thử restore luôn xóa database tạm `dbbm_verify_…` do chính nó tạo). Muốn thử lại, dùng tên mới hoặc DBA tự xóa DB lỗi sau khi xác minh.

## Phát triển và kiểm thử

Cần Go >= 1.26, Node >= 22 và client tool của loại DB cần dùng trong PATH: `pg_dump`/`pg_restore`, `mariadb-dump`/`mariadb`, `mongodump`/`mongorestore`. SQL Server không cần client tool.

```bash
cd web
npm ci
npm run build
cd ..
# Export các biến cần thiết; chương trình local không tự load .env.
export ADMIN_PASSWORD='SET_A_STRONG_LOCAL_PASSWORD'
make run
```

Sau khi đăng nhập, thêm kết nối DB local trong **Databases → Thêm kết nối**. Backend mặc định chỉ nghe `127.0.0.1:8080`. UI đã build tại http://127.0.0.1:8080. Khi phát triển React, `cd web && npm run dev`; Vite proxy `/api` tới backend.

```bash
make test
make build
```

Integration test opt-in với một PostgreSQL tạm, không dùng production:

```bash
docker run -d --name dbbackup-test -e POSTGRES_PASSWORD=integration-only-password \
  -p 127.0.0.1:55439:5432 postgres:18-bookworm
# Chờ: docker exec dbbackup-test pg_isready -U postgres
TEST_PGPORT=55439 go test -race -v ./...
docker rm -fv dbbackup-test
```

Có thể chạy tự động bằng `sh scripts/integration.sh` (tự tạo/xóa container test và chọn port trống).

Test tạo database có tên ngẫu nhiên và tự dọn; kiểm tra dữ liệu thực sau restore, từ chối target tồn tại, checksum, upload/download, xóa và scheduler callback. Unit tests kiểm tra quyền viewer, session/CSRF, xác nhận restore, cron, retention, pgpass và khôi phục trạng thái job sau restart.

## Giới hạn

- Quản lý nhiều kết nối, nhưng chỉ chạy một replica Manager trên một cặp thư mục metadata/backup. Tên DB cho job phải bắt đầu bằng chữ hoặc `_`, phần còn lại chữ/số/`_`/`-`, tối đa 63 ký tự.
- Scheduler dùng múi giờ admin chọn trong UI (mặc định lấy từ `TZ`), đổi là áp dụng ngay không cần restart; UI hiển thị timestamp theo múi giờ trình duyệt, có nhãn timezone của scheduler.
- Chưa hỗ trợ PITR/WAL, snapshot volume, backup file tăng dần, restore ghi đè hoặc quản lý globals/roles qua UI. Backup thư mục upload được hỗ trợ qua kết nối Files.
- `pg_dump` chỉ lưu một DB. Roles/tablespaces cần backup riêng bằng `pg_dumpall --globals-only`, lưu và bảo vệ riêng vì có thể chứa thông tin nhạy cảm. Logical dump không cung cấp khôi phục tại thời điểm bất kỳ (PITR). Xem [PostgreSQL pg_dump](https://www.postgresql.org/docs/current/app-pgdump.html).
- `pg_restore --list` kiểm tra mục lục archive, SHA-256 phát hiện thay đổi kể từ lúc tạo/upload; cả hai không chứng minh tính đúng đắn dữ liệu hay độ tin cậy nguồn. Restore archive có thể thực thi SQL do nguồn kiểm soát; chỉ dùng archive tin cậy. Xem [PostgreSQL pg_restore](https://www.postgresql.org/docs/current/app-pgrestore.html).
- Không bắt buộc version client/server giống nhau nhưng client dump không được cũ hơn server, và archive từ client mới có thể không đọc được bằng client cũ. Việc restore về server cũ hơn cần DBA đánh giá riêng.

## Đóng góp

Hoan nghênh issue và pull request cho sửa lỗi, cải thiện tài liệu, bản dịch hoặc tính năng mới. Với thay đổi lớn, mở issue để trao đổi phạm vi và cách triển khai trước khi viết mã.

### Báo lỗi và đề xuất

Khi báo lỗi, cung cấp phiên bản hoặc commit đang dùng, cách triển khai, loại/version database, các bước tái hiện, kết quả mong đợi và log đã ẩn thông tin riêng. Với đề xuất tính năng, mô tả nhu cầu sử dụng và hành vi mong muốn.

### Gửi pull request

1. Fork repository và tạo branch riêng cho thay đổi.
2. Giữ thay đổi tập trung; bổ sung tài liệu và kiểm thử phù hợp với hành vi thay đổi.
3. Chạy `make test`. Nếu sửa frontend, chạy `cd web && npm ci && npm run build`. Nếu sửa backup/restore PostgreSQL, chạy thêm `sh scripts/integration.sh` với Docker và PostgreSQL client trong `PATH`.
4. Mở pull request, mô tả vấn đề, thay đổi và cách đã kiểm tra. Ghi rõ các phần chưa được kiểm thử trên database thực tế.

Không đưa mật khẩu, token, khóa mã hóa, metadata, dữ liệu backup hay địa chỉ hạ tầng riêng vào issue, pull request hoặc test fixture. Dùng domain `example.com` và dữ liệu giả khi cần ví dụ.

### Cấu trúc repository

```text
cmd/server/          Điểm khởi động server và CLI
internal/manager/    API, kết nối, backup/restore, lịch, cloud và kiểm thử
web/                 Giao diện React + TypeScript
scripts/             Build/push Docker image và integration test PostgreSQL
deploy/              Compose, mẫu môi trường và hướng dẫn Portainer
Dockerfile           Build frontend, backend và các client tool
docker-compose.yml   Chạy ứng dụng từ mã nguồn
.env.example         Mẫu cấu hình cho Docker Compose
Makefile             Các lệnh phát triển và kiểm thử
```

## Bảo mật và dữ liệu riêng

Repository chỉ cung cấp cấu hình mẫu. Tự truyền tài khoản, mật khẩu, OAuth và Docker network qua môi trường hoặc file `.env` cục bộ; nhập thông tin kết nối DB, cloud và kênh thông báo trên giao diện. Khi push/deploy image, cung cấp repository của bạn qua `IMAGE`; không có registry riêng được đặt sẵn.

Các quy tắc trong `.gitignore` và `.dockerignore` loại trừ file môi trường cục bộ, khóa/chứng chỉ riêng, cấu hình rclone, thông tin đăng nhập AWS/SSH/Docker, metadata và archive backup ở các vị trí mặc định. Khi tùy chỉnh đường dẫn hoặc thêm file cấu hình, kiểm tra chúng trước khi commit. File đã được Git theo dõi không tự bị loại bỏ khi thêm quy tắc ignore.

Mật khẩu kết nối và cấu hình cloud được mã hóa bằng khóa `connections.key` cạnh metadata. Sao lưu cả metadata và khóa này để có thể khôi phục cấu hình. Bản backup cục bộ không được age mã hóa tự động; mã hóa age áp dụng trước khi upload lên cloud. Bảo vệ quyền truy cập máy chủ, thư mục dữ liệu và các khóa giải mã.

Chỉ restore archive từ nguồn tin cậy. Kiểm tra checksum và định dạng không thay thế việc kiểm tra dữ liệu sau restore. Xem [quy trình vận hành](#quy-trình-vận-hành) và [giới hạn](#giới-hạn) trước khi dùng cho dữ liệu quan trọng.

Nếu phát hiện secret đã được công khai, thu hồi hoặc đổi secret đó; xóa khỏi phiên bản hiện tại không xóa nó khỏi lịch sử Git. Khi báo vấn đề bảo mật, không đăng secret, dữ liệu thực hoặc chi tiết khai thác nhạy cảm trong issue công khai; dùng kênh báo cáo riêng của nền tảng hosting nếu repository đã bật tính năng này.

## Giấy phép

Dự án được phát hành theo giấy phép [MIT](LICENSE).
