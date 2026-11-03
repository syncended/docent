// Functions over the shell's /api endpoints (api.go).

export type Tab = "all" | "types" | "symbols" | "files" | "text" | "agent";

export interface Range {
  line: number;
  start: number;
  end: number;
}

export interface Snippet {
  startLine: number;
  lines: string[];
  matches?: Range[];
}

export interface Hit {
  tab: Tab;
  root: string;
  rootName: string;
  path: string;
  line: number;
  column: number;
  title: string;
  titleMatches?: number[];
  kind?: string;
  container?: string;
  snippet?: Snippet;
}

export interface Section {
  tab: Tab;
  hits: Hit[];
}

export interface RootInfo {
  id: string;
  name: string;
  path: string;
  files: number;
  symbols: number;
  indexedAt: string;
}

export interface Status {
  configPath: string;
  hotkey: string;
  indexing: boolean;
  roots: RootInfo[];
}

export interface Progress {
  root: string;
  done: number;
  total: number;
}

async function request<T>(method: string, path: string, params: Record<string, string>, body?: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`${path}?${new URLSearchParams(params)}`, { method, body, signal });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error ?? res.statusText);
  return data as T;
}

export const getStatus = () => request<Status>("GET", "/api/status", {});

export const searchAll = (q: string, signal?: AbortSignal) =>
  request<{ sections: Section[] }>("GET", "/api/search", { tab: "all", q }, undefined, signal).then((r) => r.sections ?? []);

export const searchTab = (tab: Exclude<Tab, "all" | "agent">, q: string, regex: boolean, matchCase: boolean, signal?: AbortSignal) =>
  request<{ hits: Hit[] }>("GET", "/api/search", { tab, q, regex: regex ? "1" : "0", case: matchCase ? "1" : "0" }, undefined, signal).then(
    (r) => r.hits ?? [],
  );

export const readFile = (root: string, path: string) =>
  request<{ content: string }>("GET", "/api/file", { root, path }).then((r) => r.content);

export const writeFile = (root: string, path: string, content: string) =>
  request<{ ok: boolean }>("PUT", "/api/file", { root, path }, content);

export const definitions = (ident: string) =>
  request<{ hits: Hit[] }>("GET", "/api/definitions", { ident }).then((r) => r.hits ?? []);

export const usages = (ident: string) =>
  request<{ hits: Hit[] }>("GET", "/api/usages", { ident }).then((r) => r.hits ?? []);

export type Layout = "compact" | "results" | "viewer";

export const setLayout = (name: Layout) => request<{ ok: boolean }>("POST", "/api/layout", { name });

export interface Root {
  path: string;
  include: string[] | null;
  exclude: string[] | null;
}

export interface Agent {
  provider: "" | "anthropic" | "openai";
  model: string;
  baseUrl: string;
}

export interface Palette {
  screen: "" | "focus" | "mouse" | "primary";
}

export interface Settings {
  configPath: string;
  hotkey: string;
  roots: Root[];
  agent: Agent;
  palette: Palette;
  hasKey: boolean;
  apiKey?: string;
  clearKey?: boolean;
}

export const getSettings = () => request<Settings>("GET", "/api/settings", {});

export const putSettings = (s: Settings) => request<Settings>("PUT", "/api/settings", {}, JSON.stringify(s));

export const pickFolder = () => request<{ path: string }>("POST", "/api/pick-folder", {}).then((r) => r.path);

export interface AgentEvent {
  kind: "turn" | "thinking" | "call" | "step" | "text" | "done" | "error";
  tool?: string;
  input?: string;
  text?: string;
  hits?: Hit[];
}

export const askAgent = (question: string) =>
  request<{ run: number }>("POST", "/api/agent/ask", {}, JSON.stringify({ question })).then((r) => r.run);

export const stopAgent = () => request<{ ok: boolean }>("POST", "/api/agent/stop", {});

export interface Span {
  from: number;
  to: number;
  class: string;
}

export const highlight = (path: string, text: string) =>
  request<{ spans: Span[] }>("POST", "/api/highlight", { path }, text).then((r) => r.spans ?? []);

export const hide = () => request<{ ok: boolean }>("POST", "/api/hide", {});

export const reindex = () => request<{ ok: boolean }>("POST", "/api/reindex", {});
