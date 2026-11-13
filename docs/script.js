const copyButton = document.querySelector("#copy-command");
const copyStatus = document.querySelector("#copy-status");

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(
      document.querySelector("#install-command").textContent,
    );
    copyStatus.textContent =
      "Copied. Remember to configure .env and directory permissions before starting.";
    copyButton.textContent = "Copied ✓";
  } catch {
    copyStatus.textContent =
      "Clipboard unavailable. You can select and copy the commands above.";
  }
});
