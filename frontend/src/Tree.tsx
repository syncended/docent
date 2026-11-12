import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ChevronDown, ChevronRight, File, Folder } from "lucide-react";
import * as api from "./api";
import type { Entry } from "./api";

// Place is a file or folder of a code root; path "" is the root itself.
export type Place = { root: string; path: string };

export interface TreeProps {
  // roots are shown as top-level folders.
  roots: { id: string; name: string }[];
  // file is the open file; its folders are expanded and it is selected.
  file: Place | null;
  // reveal expands and selects a folder; n changes on every request.
  reveal: (Place & { n: number }) | null;
  // version changes when the index is rebuilt.
  version: number;
  // focus moves keyboard focus to the tree; it changes on every request.
  focus: number;
  onOpen: (root: string, path: string) => void;
  onEscape: () => void;
}

type Row = Place & { name: string; dir: boolean; depth: number };

const key = (p: Place) => `${p.root}\0${p.path}`;

// Row geometry, matching .tree-row and .tree-body in style.css.
const rowHeight = 24;
const bodyPadding = 4;
// overscan rows are rendered beyond the view on each side.
const overscan = 10;
// sideMargin is kept around the selected row's chevron and name when
// scrolling sideways.
const sideMargin = 12;
const indent = (depth: number) => 8 + depth * 14;

// Tree shows the indexed files of code roots, loading folders as they open.
export function Tree(props: TreeProps) {
  const [dirs, setDirs] = useState<Map<string, Entry[]>>(new Map());
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [selected, setSelected] = useState(props.file ? key(props.file) : "");
  // query is the speed search: typing searches the names of visible rows.
  const [query, setQuery] = useState("");
  const host = useRef<HTMLDivElement>(null);
  const loading = useRef(new Set<string>());
  const rootsKey = props.roots.map((r) => r.id).join("\0");

  const load = (dir: Place) => {
    const k = key(dir);
    if (loading.current.has(k)) return;
    loading.current.add(k);
    api
      .listDir(dir.root, dir.path)
      .then((entries) => setDirs((d) => new Map(d).set(k, entries)))
      .catch(() => setDirs((d) => new Map(d).set(k, [])));
  };

  const expandTo = (p: Place, self: boolean) => {
    const parts = p.path ? p.path.split("/") : [];
    const chain: Place[] = [{ root: p.root, path: "" }];
    for (let i = 1; i < parts.length + (self ? 1 : 0); i++) chain.push({ root: p.root, path: parts.slice(0, i).join("/") });
    chain.forEach(load);
    setExpanded((e) => new Set([...e, ...chain.map(key)]));
    setSelected(key(p));
  };

  // Roots start expanded.
  useEffect(() => {
    loading.current.clear();
    setDirs(new Map());
    const roots = props.roots.map((r) => ({ root: r.id, path: "" }));
    roots.forEach(load);
    setExpanded(new Set(roots.map(key)));
  }, [rootsKey, props.version]);

  useEffect(() => {
    if (props.file) expandTo(props.file, false);
  }, [props.file?.root, props.file?.path, rootsKey, props.version]);

  useEffect(() => {
    if (props.reveal) expandTo(props.reveal, true);
  }, [props.reveal]);

  useEffect(() => {
    if (props.focus) host.current?.focus();
  }, [props.focus]);

  const rows = useMemo(() => {
    const rows: Row[] = [];
    const walk = (root: string, dir: string, depth: number) => {
      for (const e of dirs.get(key({ root, path: dir })) ?? []) {
        const path = dir ? `${dir}/${e.name}` : e.name;
        rows.push({ root, path, name: e.name, dir: e.dir, depth });
        if (e.dir && expanded.has(key({ root, path }))) walk(root, path, depth + 1);
      }
    };
    for (const r of props.roots) {
      rows.push({ root: r.id, path: "", name: r.name, dir: true, depth: 0 });
      if (expanded.has(key({ root: r.id, path: "" }))) walk(r.id, "", 1);
    }
    return rows;
  }, [dirs, expanded, props.roots]);

  // Only the rows in view are rendered: a folder can hold thousands of files.
  const body = useRef<HTMLDivElement>(null);
  const [view, setView] = useState({ top: 0, height: 0 });
  const measure = () => {
    const el = body.current;
    if (el) setView({ top: el.scrollTop, height: el.clientHeight });
  };
  useLayoutEffect(() => {
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(body.current!);
    return () => observer.disconnect();
  }, []);
  const first = Math.max(0, Math.floor((view.top - bodyPadding) / rowHeight) - overscan);
  const last = Math.min(rows.length, Math.ceil((view.top + view.height - bodyPadding) / rowHeight) + overscan);

  // The selected row scrolls into view both ways: deep rows sit far right.
  // Sideways it needs the rendered row, which may only appear once the
  // vertical scroll has re-rendered the rows in view.
  const sideways = useRef(false);
  useLayoutEffect(() => {
    const el = body.current;
    const i = rows.findIndex((r) => key(r) === selected);
    if (!el || i < 0) return;
    const top = bodyPadding + i * rowHeight;
    if (top < el.scrollTop) el.scrollTop = top - bodyPadding;
    else if (top + rowHeight > el.scrollTop + el.clientHeight) el.scrollTop = top + rowHeight + bodyPadding - el.clientHeight;
    sideways.current = true;
  }, [selected, rows.length]);
  useLayoutEffect(() => {
    const el = body.current;
    const row = sideways.current ? el?.querySelector<HTMLElement>(".tree-row.selected") : null;
    if (!el || !row) return;
    sideways.current = false;
    // From the row's chevron to the end of its name, in scroll coordinates.
    const origin = el.getBoundingClientRect().left - el.scrollLeft;
    const start = row.firstElementChild!.getBoundingClientRect().left - origin - sideMargin;
    const end = row.querySelector(".tree-name")!.getBoundingClientRect().right - origin + sideMargin;
    // The whole name if it fits, else as much of it as fits from its start.
    if (start < el.scrollLeft) el.scrollLeft = Math.max(0, start);
    else if (end > el.scrollLeft + el.clientWidth) el.scrollLeft = Math.max(0, Math.min(start, end - el.clientWidth));
  });

  const toggle = (p: Place, open: boolean) => {
    if (open) load(p);
    setExpanded((e) => {
      const next = new Set(e);
      if (open) next.add(key(p));
      else next.delete(key(p));
      return next;
    });
  };

  const activate = (row: Row) => {
    setSelected(key(row));
    if (row.dir) toggle(row, !expanded.has(key(row)));
    else props.onOpen(row.root, row.path);
  };

  const matches = (r: Row, q = query) => q !== "" && r.name.toLowerCase().includes(q.toLowerCase());

  // search sets the query and selects the first match from the selection on.
  const search = (q: string) => {
    setQuery(q);
    const i = Math.max(rows.findIndex((r) => key(r) === selected), 0);
    const hit = [...rows.slice(i), ...rows.slice(0, i)].find((r) => matches(r, q));
    if (hit) setSelected(key(hit));
  };

  // step selects the next or previous match.
  const step = (dir: 1 | -1) => {
    const i = rows.findIndex((r) => key(r) === selected);
    const ordered = dir > 0 ? rows.slice(i + 1) : rows.slice(0, Math.max(i, 0)).reverse();
    const hit = ordered.find((r) => matches(r));
    if (hit) setSelected(key(hit));
  };

  // onSearchKey handles typing into the speed search; true if it did.
  const onSearchKey = (e: React.KeyboardEvent): boolean => {
    if (e.ctrlKey || e.metaKey || e.altKey) return false;
    if (e.key.length === 1 && (query !== "" || e.key !== " ")) {
      search(query + e.key);
      return true;
    }
    if (query === "") return false;
    switch (e.key) {
      case "Backspace":
        search(query.slice(0, -1));
        return true;
      case "Escape":
        setQuery("");
        return true;
      case "ArrowDown":
      case "ArrowUp":
        step(e.key === "ArrowDown" ? 1 : -1);
        return true;
      case "Enter":
        setQuery("");
        return false;
    }
    return false;
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (onSearchKey(e)) {
      e.preventDefault();
      e.stopPropagation();
      return;
    }
    const i = rows.findIndex((r) => key(r) === selected);
    const row = rows[i];
    switch (e.key) {
      case "ArrowDown":
        if (rows.length) setSelected(key(rows[Math.min(i + 1, rows.length - 1)]));
        break;
      case "ArrowUp":
        if (rows.length) setSelected(key(rows[Math.max(i - 1, 0)]));
        break;
      case "ArrowRight":
        if (row?.dir && !expanded.has(key(row))) toggle(row, true);
        else if (row?.dir && rows[i + 1]?.depth > row.depth) setSelected(key(rows[i + 1]));
        break;
      case "ArrowLeft":
        if (row?.dir && expanded.has(key(row))) toggle(row, false);
        else if (row && row.path) setSelected(key({ root: row.root, path: row.path.slice(0, Math.max(row.path.lastIndexOf("/"), 0)) }));
        break;
      case "Enter":
        if (row) activate(row);
        break;
      case "Escape":
        props.onEscape();
        break;
      default:
        return;
    }
    e.preventDefault();
    e.stopPropagation();
  };

  const open = props.file ? key(props.file) : "";
  const found = query !== "" && rows.some((r) => matches(r));
  return (
    <div className="tree" ref={host} tabIndex={0} onKeyDown={onKeyDown} onBlur={() => setQuery("")}>
      {query && <div className={found ? "tree-search" : "tree-search none"}>{query}</div>}
      <div className="tree-body" ref={body} onScroll={measure}>
        <div className="tree-rows" style={{ height: rows.length * rowHeight, paddingTop: first * rowHeight }}>
        {rows.slice(first, last).map((row) => (
          <div
            key={key(row)}
            className={`tree-row${key(row) === selected ? " selected" : key(row) === open ? " open" : ""}${row.depth === 0 ? " tree-root" : ""}`}
            style={{ paddingLeft: indent(row.depth) }}
            onMouseDown={(e) => (e.preventDefault(), host.current?.focus())}
            onClick={() => activate(row)}
          >
            {row.dir ? (
              expanded.has(key(row)) ? <ChevronDown size={13} /> : <ChevronRight size={13} />
            ) : (
              <span className="tree-indent" />
            )}
            {row.dir ? <Folder size={13} /> : <File size={13} />}
            <span className="tree-name">{matches(row) ? <Match name={row.name} query={query} /> : row.name}</span>
          </div>
        ))}
        </div>
      </div>
    </div>
  );
}

// Match shows a name with the speed search query highlighted.
function Match({ name, query }: { name: string; query: string }) {
  const i = name.toLowerCase().indexOf(query.toLowerCase());
  return (
    <>
      {name.slice(0, i)}
      <mark>{name.slice(i, i + query.length)}</mark>
      {name.slice(i + query.length)}
    </>
  );
}
