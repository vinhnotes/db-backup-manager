# Trang giới thiệu trên GitHub Pages

Trang tĩnh tiếng Việt nằm trong `docs/`, gồm HTML, CSS, JavaScript, favicon và ảnh chụp màn hình trong `docs/images/`. Không cần cài dependency hay build frontend React. Ảnh chụp là ứng dụng thật chạy với dữ liệu demo (Postgres tạm, không chứa dữ liệu thật).

## Xuất bản lần đầu

1. Commit và push các file `docs/`, `.github/workflows/pages.yml` cùng tài liệu lên nhánh `main` của repository.
2. Trong repository GitHub, mở **Settings → Pages → Build and deployment → Source**, chọn **GitHub Actions**.
3. Mở **Actions → Deploy product page → Run workflow**, chọn nhánh `main` và chạy. Các lần push tiếp theo có thay đổi trong `docs/` hoặc workflow sẽ tự triển khai.
4. Khi workflow thành công, truy cập **https://vinhnotes.github.io/db-backup-manager/**. URL thực tế cũng xuất hiện trong environment `github-pages` của lần chạy.

Workflow chỉ upload `docs/`. Không đưa mã backend, dữ liệu backup, cấu hình môi trường hay ứng dụng quản trị lên Pages. Trang này giới thiệu sản phẩm; ứng dụng quản lý backup cần được tự triển khai riêng.

Nếu fork repository, thay các URL `vinhnotes/db-backup-manager` trong `docs/index.html` và tài liệu bằng repository của bạn. Asset sử dụng đường dẫn tương đối, tương thích địa chỉ Pages có tiền tố tên repository.

## Xem thử tại máy

Từ thư mục gốc dự án:

```sh
python3 -m http.server 4173 --bind 127.0.0.1 --directory docs
```

Mở **http://127.0.0.1:4173**. Chỉnh sửa nội dung trong `docs/index.html`, giao diện trong `docs/styles.css` và xử lý sao chép lệnh trong `docs/script.js`.

Tham khảo: [GitHub — Using custom workflows with GitHub Pages](https://docs.github.com/en/pages/getting-started-with-github-pages/using-custom-workflows-with-github-pages).
