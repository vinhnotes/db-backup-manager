import { Fragment, ReactNode, useSyncExternalStore } from "react";
import vi from "./locales/vi";
import en from "./locales/en";
import ja from "./locales/ja";
import serverMessages from "./locales/server";

export type Key = keyof typeof vi;
export const languages = [
  ["vi", "Tiếng Việt"],
  ["en", "English"],
  ["ja", "日本語"],
] as const;
export type Lang = (typeof languages)[number][0];

const dicts: Record<Lang, Record<Key, string>> = { vi, en, ja };
const locales: Record<Lang, string> = {
  vi: "vi-VN",
  en: "en-US",
  ja: "ja-JP",
};
const STORAGE_KEY = "lang";

function detect(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved && saved in dicts) return saved as Lang;
  } catch {
    // storage unavailable (private mode); fall through to the browser language
  }
  for (const l of navigator.languages ?? [navigator.language]) {
    const base = l.toLowerCase().split("-")[0];
    if (base in dicts) return base as Lang;
  }
  return "en";
}

let current: Lang = detect();
document.documentElement.lang = current;
const listeners = new Set<() => void>();

export const getLang = () => current;
/** BCP 47 locale for Intl/toLocaleString. */
export const locale = () => locales[current];

export function setLang(lang: Lang) {
  if (lang === current) return;
  current = lang;
  document.documentElement.lang = lang;
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    // ignore
  }
  listeners.forEach((f) => f());
}

/** Re-renders the caller when the language changes. */
export function useLang() {
  return useSyncExternalStore((cb) => {
    listeners.add(cb);
    return () => listeners.delete(cb);
  }, getLang);
}

type Vars = Record<string, string | number>;

/** Replaces {name} and plural forms {name|one|other}. */
function fill(s: string, vars: Vars = {}) {
  return s.replace(
    /\{(\w+)(?:\|([^|}]*)\|([^}]*))?\}/g,
    (m, name: string, one?: string, other?: string) => {
      const v = vars[name];
      if (v === undefined) return m;
      if (one !== undefined)
        return new Intl.PluralRules(locales[current]).select(Number(v)) ===
          "one"
          ? one
          : other!;
      return String(v);
    },
  );
}

/** Translated plain string. */
export const t = (key: Key, vars?: Vars) =>
  fill(dicts[current][key] ?? vi[key], vars);

type Wrap = (chunk: string) => ReactNode;
type RichVars = Record<string, ReactNode | Wrap>;
const defaultTags: Record<string, Wrap> = {
  b: (c) => <strong>{c}</strong>,
  code: (c) => <code>{c}</code>,
};

/**
 * Translated rich text. <b>, <code> and any tag passed in `vars` as a
 * function wrap their content; {name} is replaced by `vars[name]`, which
 * may be a React node.
 */
export function tx(key: Key, vars: RichVars = {}): ReactNode {
  const src = dicts[current][key] ?? vi[key];
  const plain: Vars = {};
  for (const [k, v] of Object.entries(vars))
    if (typeof v === "string" || typeof v === "number") plain[k] = v;
  const out: ReactNode[] = [];
  const re = /<(\w+)>([\s\S]*?)<\/\1>|\{(\w+)\}/g;
  let last = 0;
  let m: RegExpExecArray | null;
  while ((m = re.exec(src))) {
    if (m.index > last) out.push(fill(src.slice(last, m.index), plain));
    let node: ReactNode;
    if (m[1]) {
      const wrap = vars[m[1]] ?? defaultTags[m[1]];
      const inner = fill(m[2], plain);
      node = typeof wrap === "function" ? (wrap as Wrap)(inner) : inner;
    } else {
      const v = vars[m[3]];
      node = v === undefined || typeof v === "function" ? m[0] : v;
    }
    out.push(<Fragment key={out.length}>{node}</Fragment>);
    last = re.lastIndex;
  }
  if (last < src.length) out.push(fill(src.slice(last), plain));
  return out;
}

// Server error messages are written in Vietnamese; translate known ones.
const compiled = serverMessages.map(([source, en, ja]) => {
  const parts = source
    .split(/\{\d\}/)
    .map((p) => p.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  // A placeholder at the very end takes the rest of the line.
  const pattern = parts.reduce(
    (acc, p, i) =>
      acc + (i === parts.length - 1 && p === "" ? "(.*)" : "(.*?)") + p,
  );
  return { re: new RegExp(pattern, "g"), en, ja };
});

/** Translates Vietnamese server messages (also inside longer job errors). */
export function translateServer(message: string) {
  if (current === "vi" || !message) return message;
  let out = message;
  for (const c of compiled)
    out = out.replace(c.re, (...m) =>
      c[current as "en" | "ja"].replace(
        /\{(\d)\}/g,
        (_, i) => m[Number(i) + 1],
      ),
    );
  return out;
}
