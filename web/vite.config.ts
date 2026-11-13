import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
export default defineConfig({
  plugins: [react()],
  // server_messages.json is shared with the Go server in ../internal.
  server: { proxy: { "/api": "http://127.0.0.1:8080" }, fs: { allow: [".", "../internal/manager"] } },
});
