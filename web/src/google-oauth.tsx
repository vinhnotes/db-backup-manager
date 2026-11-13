import { useEffect, useState } from "react";
import { api } from "./lib";
import { t, translateServer } from "./i18n";

type OAuthSettings = { available: boolean; redirect_url: string };

export function GoogleDriveOAuth({
  ready,
  connected,
  onReady,
  onPending,
}: {
  ready: boolean;
  connected: boolean;
  onReady: (id: string) => void;
  onPending: (pending: boolean) => void;
}) {
  const [settings, setSettings] = useState<OAuthSettings | null>(null);
  const [scope, setScope] = useState("drive.file");
  const [flow, setFlow] = useState("");
  const [error, setError] = useState("");
  const [starting, setStarting] = useState(false);
  useEffect(() => {
    onPending(starting || !!flow);
    return () => onPending(false);
  }, [starting, flow, onPending]);
  useEffect(() => {
    let active = true;
    api<OAuthSettings>("/destinations/google/oauth")
      .then((value) => {
        if (active) setSettings(value);
      })
      .catch((e) => {
        if (active) setError((e as Error).message);
      });
    return () => {
      active = false;
    };
  }, []);
  useEffect(() => {
    if (!flow) return;
    let active = true;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const result = await api<{ ready: boolean; error: string }>(
          "/destinations/google/oauth/status?id=" + encodeURIComponent(flow),
        );
        if (!active) return;
        if (result.error) {
          setError(translateServer(result.error));
          setFlow("");
        } else if (result.ready) {
          onReady(flow);
          setFlow("");
        } else timer = setTimeout(() => void poll(), 1500);
      } catch (e) {
        if (active) {
          setError((e as Error).message);
          setFlow("");
        }
      }
    };
    timer = setTimeout(() => void poll(), 1000);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [flow, onReady]);
  async function connect() {
    // Open synchronously so browsers allow the login window. No opener is
    // needed: the parent polls authenticated status and never reads tokens.
    const popup = window.open(
      "about:blank",
      "_blank",
      "popup,width=560,height=760",
    );
    if (!popup) {
      setError(t("googleOAuth.popup"));
      return;
    }
    popup.opener = null;
    onReady("");
    setStarting(true);
    setError("");
    try {
      const result = await api<{ id: string; url: string }>(
        "/destinations/google/oauth/start",
        "POST",
        { scope },
      );
      popup.location.href = result.url;
      setFlow(result.id);
    } catch (e) {
      popup.close();
      setError((e as Error).message);
    } finally {
      setStarting(false);
    }
  }
  return (
    <div className="form-note steps">
      <p>{t("googleOAuth.description")}</p>
      {settings && !settings.available && <p>{t("googleOAuth.setup")}</p>}
      {settings?.redirect_url && (
        <p>
          {t("googleOAuth.callback")} <code>{settings.redirect_url}</code>
        </p>
      )}
      <p>{t("googleOAuth.tunnel")}</p>
      <label>
        {t("googleOAuth.scope")}
        <select
          value={scope}
          disabled={starting || !!flow}
          onChange={(e) => {
            setScope(e.target.value);
            onReady("");
          }}
        >
          <option value="drive.file">{t("googleOAuth.scopeFile")}</option>
          <option value="drive">{t("googleOAuth.scopeFull")}</option>
        </select>
        <small>{t("googleOAuth.scopeHint")}</small>
      </label>
      {ready && <p role="status">{t("googleOAuth.ready")}</p>}
      {error && (
        <p className="alert" role="alert">
          {error}
        </p>
      )}
      <button
        type="button"
        className="secondary"
        disabled={!settings?.available || starting || !!flow}
        onClick={() => void connect()}
      >
        {flow || starting
          ? t("googleOAuth.waiting")
          : ready || connected
            ? t("googleOAuth.reconnect")
            : t("googleOAuth.connect")}
      </button>
      {flow && (
        <button type="button" className="secondary" onClick={() => setFlow("")}>
          {t("common.cancel")}
        </button>
      )}
    </div>
  );
}
