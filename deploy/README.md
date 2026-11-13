# Deploy qua Portainer

Tài liệu dành cho Docker Standalone quản lý bằng Portainer. Không dùng được cho Swarm.

| File | Dùng để |
|---|---|
| [docker-compose.portainer.yml](docker-compose.portainer.yml) | Nội dung stack dán vào Portainer |
| [stack.env.example](stack.env.example) | Mẫu biến môi trường cho stack |

Image do bạn tự build và cung cấp: `${IMAGE}:${IMAGE_TAG}`. `IMAGE` là repository không kèm tag; dự án không đặt sẵn registry.

## 1. Build và push image (máy dev)

```sh
# Thay địa chỉ registry và namespace bên dưới bằng thông tin của bạn.
export IMAGE=registry.example.com/your-namespace/db-backup-manager
docker login registry.example.com
make docker-push TAG=v1.0.0        # push :v1.0.0 và :latest
```

Có thể truyền repository trực tiếp: `./scripts/docker-push.sh v1.0.0 "$IMAGE"`. Script không tự đọc `.env`; hãy export `IMAGE` hoặc truyền qua dòng lệnh. Không truyền tag trong `IMAGE`. Nếu không chỉ định tag, script dùng git short SHA (thêm `-dirty` khi có thay đổi chưa commit). Với Docker Hub, dùng `IMAGE=your-namespace/db-backup-manager` và `docker login`.

Để đăng nhập tự động, truyền đồng thời `REGISTRY_USER` và `REGISTRY_PASSWORD` qua môi trường (ví dụ secret của CI). `REGISTRY` mặc định được suy ra từ `IMAGE`, hoặc `docker.io` với Docker Hub; có thể đặt riêng nếu cần. Không lưu thông tin đăng nhập trong repository.

Mặc định build `linux/amd64`. Đổi bằng `PLATFORMS=linux/arm64 make docker-push ...` nếu server là ARM.

## 2. Khai báo registry trong Portainer (một lần)

**Registries → Add registry → Custom registry**

- Name: tên gợi nhớ do bạn đặt
- Registry URL: hostname registry của bạn (ví dụ `registry.example.com`)
- Nếu dùng private registry, bật **Authentication** và nhập tài khoản registry của bạn. Image public không cần thông tin đăng nhập.

## 3. Tạo stack

**Stacks → Add stack**

1. Name: `db-backup-manager`
2. Build method:
   - **Web editor**: dán nội dung [docker-compose.portainer.yml](docker-compose.portainer.yml), hoặc
   - **Repository**: trỏ tới repo, Compose path `deploy/docker-compose.portainer.yml`.
3. **Environment variables → Advanced mode**: dán [stack.env.example](stack.env.example) rồi điền tối thiểu:
   - `IMAGE`: repository vừa push, không kèm tag (giống bước 1)
   - `IMAGE_TAG`: tag vừa push, ví dụ `v1.0.0`
   - `ADMIN_PASSWORD`: ít nhất 12 ký tự
   - `POSTGRES_NETWORK`: tên network của stack PostgreSQL (xem bằng `docker network ls` trên server)
4. **Deploy the stack**.

Không cần tạo thư mục hay `chown` trước. Xem mục [Lưu trữ dữ liệu](#lưu-trữ-dữ-liệu).

## 4. Kiểm tra và truy cập

- **Containers → db-backup-manager** phải chuyển sang `healthy` sau 10–30 giây. Nếu không, xem **Logs**.
- Port chỉ mở trên `127.0.0.1`. Truy cập từ máy cá nhân bằng SSH tunnel:

  ```sh
  ssh -N -L 8088:127.0.0.1:8088 user@SERVER_IP
  ```

  rồi mở **https://localhost:8088** (gõ `http://` cũng tự chuyển sang `https://`). Trình duyệt cảnh báo chứng chỉ ở lần đầu (self-signed, tự sinh trong volume data): chọn **Advanced → Proceed**.
- Nếu mở trực tiếp bằng IP/tên miền server, thêm chúng vào `TLS_HOSTS` để chứng chỉ khớp địa chỉ. Chi tiết: [README → HTTPS](../README.md#https).
- Khi thêm kết nối DB trên giao diện, host là **tên service/container Postgres** trong network chung (ví dụ `postgres`), không phải `localhost`.

## Lưu trữ dữ liệu

Mặc định (để trống `DATA_DIR`, `BACKUP_DIR`), stack dùng hai **named volume**:

| Volume | Mount | Nội dung |
|---|---|---|
| `db-backup-manager-data` | `/data` | `metadata.db`, `connections.key`, `tls/` (chứng chỉ HTTPS) |
| `db-backup-manager-backups` | `/backups` | File backup `.dump` |

Docker tự tạo volume ở lần deploy đầu và sao chép quyền sở hữu từ image (user `postgres`, UID 999), nên container ghi được ngay. Xóa hay deploy lại stack **không** xóa volume. Chỉ mất khi xóa volume thủ công trong **Volumes**.

Vị trí thật trên server: `/var/lib/docker/volumes/<tên volume>/_data`.

### Dùng thư mục trên host (tùy chọn)

Chỉ cần khi muốn backup nằm ở ổ/thư mục cụ thể. Đặt `DATA_DIR`, `BACKUP_DIR` thành **đường dẫn tuyệt đối** và tạo trước:

```sh
sudo install -d -o 999 -g 999 -m 700 /opt/db-backup-manager/data /opt/backups/db
```

Phải tạo trước vì với bind mount, Docker tự tạo thư mục còn thiếu nhưng thuộc `root`, trong khi container chạy bằng UID 999 nên không ghi được (`permission denied` khi mở `metadata.db`).

Nếu đã gán 999 mà vẫn lỗi, kiểm tra:

- **Đường dẫn tương đối** (`./data`): trong Portainer, đường dẫn này không trỏ tới thư mục bạn đã `chown`. Luôn dùng đường dẫn tuyệt đối.
- **File cũ bên trong** thuộc user khác (copy từ máy khác, từng chạy bằng root): `chown` phải có `-R`.

  ```sh
  sudo chown -R 999:999 /opt/db-backup-manager/data /opt/backups/db
  ```

- **Thư mục cha** không cho UID 999 đi qua (ví dụ thư mục cha có quyền `700` thuộc root).
- **Docker rootless hoặc `userns-remap`**: UID trong container được ánh xạ sang UID khác trên host. Khi đó nên dùng named volume.
- **SELinux** (RHEL/CentOS/Rocky): thêm hậu tố `:Z` vào volume, ví dụ `/opt/backups/db:/backups:Z`.

## Cập nhật phiên bản

1. Máy dev: `IMAGE=registry.example.com/your-namespace/db-backup-manager make docker-push TAG=v1.0.1` (thay bằng repository của bạn)
2. Portainer: **Stacks → db-backup-manager → Editor**, đổi `IMAGE_TAG=v1.0.1` → **Update the stack**.
   - Nếu dùng `latest`, bật **Re-pull image and redeploy**.

Dữ liệu trong volume được giữ nguyên.

## Sao lưu chính công cụ

Sao lưu **cả** `metadata.db` và `connections.key` (trong volume data). Mất `connections.key` thì không giải mã được mật khẩu kết nối và cấu hình cloud đã lưu.

```sh
docker run --rm -v db-backup-manager-data:/data:ro -v "$PWD":/out debian:bookworm-slim \
  tar czf /out/dbbm-data-$(date +%F).tgz -C /data .
```

## Xử lý sự cố

- **Quên mật khẩu admin**: **Containers → db-backup-manager → Console**, chạy `db-backup-manager reset-admin-password`.
- **`network postgres_network not found`**: sai `POSTGRES_NETWORK`. Kiểm tra lại bằng `docker network ls`.
- **`pull access denied` / `unauthorized`**: chưa khai báo registry (bước 2) hoặc sai tài khoản.
- **Chạy sau reverse proxy HTTPS**: đặt `TLS_MODE=off`, `COOKIE_SECURE=true` và cho proxy kết nối tới port của container.
