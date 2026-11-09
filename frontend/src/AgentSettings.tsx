import { useEffect, useMemo, useRef, useState } from "react";
import * as api from "./api";

// AgentDraft is the Agent tab as edited: the settings, plus keys typed and
// keys to remove, by provider, which go to the credential store on save.
export type AgentDraft = {
  agent: api.Agent;
  keys: Record<string, string>;
  clearKeys: Set<string>;
};

const groups: { kind: api.Provider["kind"]; label: string }[] = [
  { kind: "vendor", label: "Model vendors" },
  { kind: "host", label: "Hosts and aggregators" },
  { kind: "local", label: "Local servers" },
  { kind: "custom", label: "Other" },
];

const levelLabels: Record<api.Level, string> = { low: "Low", medium: "Medium", high: "High" };

const empty: api.ProviderSettings = { model: "", effort: "", endpoint: "", baseUrl: "" };

// tokens renders a token count as 128K or 1M.
export function tokens(n: number | undefined): string {
  if (!n) return "";
  const [unit, suffix] = n >= 1_000_000 ? [n % 1048576 === 0 ? 1048576 : 1e6, "M"] : [n % 1024 === 0 ? 1024 : 1000, "K"];
  return `${+(n / unit).toFixed(1)}${suffix}`;
}

// score is a fuzzy match of query against text: the letters of the query in
// order, earlier and contiguous matches scoring higher; -1 if no match.
function score(query: string, text: string): number {
  query = query.toLowerCase();
  text = text.toLowerCase();
  if (!query) return 0;
  const at = text.indexOf(query);
  if (at >= 0) return 1000 - at;
  let s = 0;
  let i = 0;
  let prev = -2;
  for (let j = 0; j < text.length && i < query.length; j++) {
    if (text[j] === query[i]) {
      s += j === prev + 1 ? 5 : 1;
      prev = j;
      i++;
    }
  }
  return i === query.length ? s : -1;
}

export function AgentSettings(props: {
  providers: api.Provider[];
  levels: api.Level[];
  defaultLevel: api.Level;
  // stored lists the providers with a key in the credential store.
  stored: string[];
  draft: AgentDraft;
  onChange: (d: AgentDraft) => void;
}) {
  const { providers, draft, onChange } = props;
  const { agent } = draft;
  const p = providers.find((x) => x.id === agent.provider) ?? providers[0];
  const s = { ...empty, ...agent.providers[p?.id ?? ""] };
  const endpoint = p?.endpoints.find((e) => e.id === s.endpoint) ?? p?.endpoints[0];
  const typedKey = draft.keys[p?.id ?? ""] ?? "";
  const keyStored = props.stored.includes(p?.id ?? "") && !draft.clearKeys.has(p?.id ?? "");
  const hasKey = (id: string) => !!draft.keys[id] || (props.stored.includes(id) && !draft.clearKeys.has(id));

  const setSettings = (id: string, change: Partial<api.ProviderSettings>) =>
    onChange({ ...draft, agent: { ...agent, providers: { ...agent.providers, [id]: { ...empty, ...agent.providers[id], ...change } } } });

  // The model list follows the provider, its address and key.
  const [list, setList] = useState<{ models: api.ModelChoice[]; live: boolean; error?: string; loading: boolean }>({ models: [], live: false, loading: true });
  const base = s.baseUrl.trim();
  useEffect(() => {
    if (!p || !endpoint) return;
    const ctl = new AbortController();
    setList((l) => ({ ...l, loading: true }));
    const t = setTimeout(() => {
      api.listModels({ provider: p.id, endpoint: endpoint.id, baseUrl: base, apiKey: typedKey.trim() }, ctl.signal).then(
        (r) => setList({ models: r.models ?? [], live: r.live, error: r.error, loading: false }),
        (e) => {
          if (!ctl.signal.aborted) setList({ models: [], live: false, error: (e as Error).message, loading: false });
        },
      );
    }, 400);
    return () => {
      clearTimeout(t);
      ctl.abort();
    };
  }, [p?.id, endpoint?.id, base, typedKey]);

  // A known provider's address typed into Custom switches to its preset,
  // with the key typed (DESIGN.md, Providers and models).
  const [recognized, setRecognized] = useState("");
  useEffect(() => {
    if (p?.kind !== "custom" || !base) return;
    let live = true;
    const t = setTimeout(() => {
      api.matchProvider(base).then((m) => {
        if (!live || !m.provider) return;
        const keys = { ...draft.keys };
        if (keys[p.id]) {
          keys[m.provider] = keys[p.id];
          delete keys[p.id];
        }
        const target = { ...empty, ...agent.providers[m.provider], endpoint: m.endpoint ?? "" };
        onChange({
          ...draft,
          keys,
          agent: { provider: m.provider, providers: { ...agent.providers, [p.id]: { ...s, baseUrl: "" }, [m.provider]: target } },
        });
        setRecognized(providers.find((x) => x.id === m.provider)?.name ?? "");
      }, () => {});
    }, 500);
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [p?.id, base]);

  if (!p || !endpoint) return null;

  const model = s.model || p.recommended || "";
  const chosen: api.ModelChoice | undefined = list.models.find((m) => m.id === model);
  const level = s.effort || props.defaultLevel;

  return (
    <div className="agent-settings">
      <nav className="provider-list">
        {groups.map((g) => (
          <div key={g.kind} className="provider-group">
            <h4>{g.label}</h4>
            {providers
              .filter((x) => x.kind === g.kind)
              .map((x) => (
                <button
                  key={x.id}
                  className={x.id === p.id ? "provider active" : "provider"}
                  onClick={() => {
                    setRecognized("");
                    onChange({ ...draft, agent: { ...agent, provider: x.id } });
                  }}
                >
                  <span>{x.name}</span>
                  {hasKey(x.id) && <span className="provider-key" title="API key stored" />}
                </button>
              ))}
          </div>
        ))}
      </nav>
      <div className="provider-detail">
        {recognized && <p className="hint">Recognized the address as {recognized}.</p>}
        {p.endpoints.length > 1 && (
          <div className="pills">
            {p.endpoints.map((e) => (
              <button key={e.id} className={e.id === endpoint.id ? "pill active" : "pill"} onClick={() => setSettings(p.id, { endpoint: e.id })}>
                {e.label}
              </button>
            ))}
          </div>
        )}
        <label>
          <span>Base URL</span>
          <input
            className="field"
            placeholder={endpoint.url || "https://llm.example.com/v1"}
            value={s.baseUrl}
            onChange={(e) => setSettings(p.id, { baseUrl: e.target.value })}
          />
        </label>
        {endpoint.key !== "none" && (
          <label>
            <span>API key</span>
            <input
              className="field"
              type="password"
              autoComplete="off"
              placeholder={
                keyStored
                  ? "Stored in the system credential store · paste to replace"
                  : endpoint.key === "optional"
                    ? "Optional"
                    : "Paste API key"
              }
              value={typedKey}
              onChange={(e) => onChange({ ...draft, keys: { ...draft.keys, [p.id]: e.target.value } })}
            />
            {props.stored.includes(p.id) && !typedKey && (
              <button
                className="link"
                onClick={() => {
                  const clearKeys = new Set(draft.clearKeys);
                  if (clearKeys.has(p.id)) clearKeys.delete(p.id);
                  else clearKeys.add(p.id);
                  onChange({ ...draft, clearKeys });
                }}
              >
                {draft.clearKeys.has(p.id) ? "Keep key" : "Remove key"}
              </button>
            )}
          </label>
        )}
        {draft.clearKeys.has(p.id) && <p className="hint">The stored key will be removed on save.</p>}
        <ModelPicker
          key={p.id + endpoint.id}
          list={list}
          model={model}
          chosen={chosen}
          recommended={p.recommended}
          providerName={p.name}
          onPick={(id) => setSettings(p.id, { model: id === p.recommended ? "" : id })}
        />
        {chosen?.tunable && (
          <div className="setting-row">
            <span>Reasoning</span>
            <div className="pills inline">
              {props.levels.map((l) => (
                <button
                  key={l}
                  className={l === level ? "pill active" : "pill"}
                  title={l === props.defaultLevel ? "Docent's default for code search" : undefined}
                  onClick={() => setSettings(p.id, { effort: l === props.defaultLevel ? "" : l })}
                >
                  {levelLabels[l]}
                  {l === props.defaultLevel && <span className="pill-note">default</span>}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function ModelPicker(props: {
  list: { models: api.ModelChoice[]; live: boolean; error?: string; loading: boolean };
  model: string;
  chosen?: api.ModelChoice;
  recommended?: string;
  providerName: string;
  onPick: (id: string) => void;
}) {
  const { list, model } = props;
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);
  const rows = useRef<HTMLDivElement>(null);

  const q = query.trim();
  const matched = useMemo(() => {
    if (!q) return list.models;
    return list.models
      .map((m) => ({ m, s: Math.max(score(q, m.id), score(q, m.name ?? "")) }))
      .filter((x) => x.s >= 0)
      .sort((a, b) => b.s - a.s)
      .map((x) => x.m);
  }, [list.models, q]);
  // A typed id that is not listed can be used as is.
  const custom = q && !list.models.some((m) => m.id === q) ? q : "";
  const count = matched.length + (custom ? 1 : 0);

  useEffect(() => setCursor(0), [q]);
  useEffect(() => {
    rows.current?.querySelector(".model-row.cursor")?.scrollIntoView({ block: "nearest" });
  }, [cursor]);

  const pick = (id: string) => {
    props.onPick(id);
    setQuery("");
  };
  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setCursor((c) => (count ? (c + (e.key === "ArrowDown" ? 1 : count - 1)) % count : 0));
    } else if (e.key === "Enter" && count) {
      e.preventDefault();
      pick(custom && cursor === 0 ? custom : matched[cursor - (custom ? 1 : 0)].id);
    }
  };

  const status = list.loading
    ? "Loading models…"
    : list.live
      ? `Models listed by ${props.providerName}`
      : list.error
        ? `Docent's catalog · ${props.providerName} could not be reached: ${list.error}`
        : "Docent's catalog · enter a key to list the models it can use";

  return (
    <div className="model-picker">
      <div className="setting-row">
        <span>Model</span>
        <div className="model-current">
          <span className="model-name">{props.chosen?.name || model || "None"}</span>
          {props.chosen?.name && <span className="model-id">{model}</span>}
          {model && !props.chosen && !list.loading && <span className="model-tag">not listed</span>}
        </div>
      </div>
      <input
        className="field"
        placeholder="Filter models or type an id"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={onKeyDown}
      />
      <div className="model-rows" ref={rows}>
        {custom && (
          <button className={cursor === 0 ? "model-row cursor" : "model-row"} onClick={() => pick(custom)}>
            <span className="model-name">Use “{custom}”</span>
          </button>
        )}
        {matched.map((m, i) => {
          const index = i + (custom ? 1 : 0);
          return (
            <button
              key={m.id}
              className={["model-row", index === cursor && "cursor", m.id === model && "active"].filter(Boolean).join(" ")}
              onClick={() => pick(m.id)}
              onMouseMove={() => setCursor(index)}
            >
              <span className="model-name">{m.name || m.id}</span>
              {m.name && <span className="model-id">{m.id}</span>}
              <span className="model-tags">
                {m.id === props.recommended && <span className="model-tag accent">recommended</span>}
                {m.reasoning && <span className="model-tag">reasoning</span>}
                {m.context ? <span className="model-tag">{tokens(m.context)}</span> : null}
              </span>
            </button>
          );
        })}
        {!list.loading && count === 0 && <p className="hint">No models{q ? " match" : ""}.</p>}
      </div>
      <p className="hint">{status}</p>
    </div>
  );
}
