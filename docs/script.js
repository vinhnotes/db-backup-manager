const copyButton = document.querySelector("#copy-command");
const copyStatus = document.querySelector("#copy-status");

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(
      document.querySelector("#install-command").textContent,
    );
    copyStatus.textContent =
      "Đã sao chép. Nhớ cấu hình .env và quyền thư mục trước khi khởi chạy.";
    copyButton.textContent = "Đã sao chép ✓";
  } catch {
    copyStatus.textContent =
      "Không thể truy cập clipboard. Bạn có thể chọn và sao chép các lệnh bên trên.";
  }
});
