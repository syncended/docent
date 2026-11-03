import { useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronRight, File, Folder } from "lucide-react";
import * as api from "./api";
import type { Entry } from "./api";

export interface TreeProps {
  root: string;
  rootName: string;
  // path is the open file; its folders are expanded and it is selected.
  path: string;
  // reveal expands and selects a folder; n changes on every request.
  reveal: { path: string; n: number } | null;
  // version changes when the index is rebuilt.
  version: number;
  // focus moves keyboard focus to the tree; it changes on every request.
  focus: number;
  onOpen: (path: string) => void;
  onEscape: () => void;
}

type Row = { path: string; name: string; dir: boolean; depth: number };

// Tree shows the indexed files of a code root, loading folders as they open.
export function Tree(props: TreeProps) {
  const [dirs, setDirs] = useState<Map<string, Entry[]>>(new Map());
  const [expanded, setExpanded] = useState<Set<string>>(new Set([""]));
  const [selected, setSelected] = useState(props.path);
  const host = useRef<HTMLDivElement>(null);
  const loading = useRef(new Set<string>());

  const load = (dir: string) => {
    if (loading.current.has(dir)) return;
    loading.current.add(dir);
    api
      .listDir(props.root, dir)
      .then((entries) => setDirs((d) => new Map(d).set(dir, entries)))
      .catch(() => setDirs((d) => new Map(d).set(dir, [])));
  };

  const expandTo = (path: string, self: boolean) => {
    const parts = path.split("/");
    const chain = [""];
    for (let i = 1; i < parts.length + (self ? 1 : 0); i++) chain.push(parts.slice(0, i).join("/"));
    chain.forEach(load);
    setExpanded((e) => new Set([...e, ...chain]));
    setSelected(path);
  };

  useEffect(() => {
    loading.current.clear();
    setDirs(new Map());
    setExpanded(new Set([""]));
  }, [props.root, props.version]);

  useEffect(() => expandTo(props.path, false), [props.root, props.path, props.version]);

  useEffect(() => {
    if (props.reveal) expandTo(props.reveal.path, true);
  }, [props.reveal]);

  useEffect(() => {
    if (props.focus) host.current?.focus();
  }, [props.focus]);

  const rows: Row[] = [];
  const walk = (dir: string, depth: number) => {
    for (const e of dirs.get(dir) ?? []) {
      const path = dir ? `${dir}/${e.name}` : e.name;
      rows.push({ path, name: e.name, dir: e.dir, depth });
      if (e.dir && expanded.has(path)) walk(path, depth + 1);
    }
  };
  walk("", 0);

  useEffect(() => {
    host.current?.querySelector(".tree-row.selected")?.scrollIntoView({ block: "nearest" });
  }, [selected, rows.length]);

  const toggle = (path: string, open: boolean) => {
    if (open) load(path);
    setExpanded((e) => {
      const next = new Set(e);
      if (open) next.add(path);
      else next.delete(path);
      return next;
    });
  };

  const activate = (row: Row) => {
    setSelected(row.path);
    if (row.dir) toggle(row.path, !expanded.has(row.path));
    else props.onOpen(row.path);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    const i = rows.findIndex((r) => r.path === selected);
    const row = rows[i];
    const parent = selected.includes("/") ? selected.slice(0, selected.lastIndexOf("/")) : "";
    switch (e.key) {
      case "ArrowDown":
        setSelected(rows[Math.min(i + 1, rows.length - 1)]?.path ?? selected);
        break;
      case "ArrowUp":
        setSelected(rows[Math.max(i - 1, 0)]?.path ?? selected);
        break;
      case "ArrowRight":
        if (row?.dir && !expanded.has(row.path)) toggle(row.path, true);
        else if (row?.dir) setSelected(rows[i + 1]?.depth > row.depth ? rows[i + 1].path : selected);
        break;
      case "ArrowLeft":
        if (row?.dir && expanded.has(row.path)) toggle(row.path, false);
        else if (parent) setSelected(parent);
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

  return (
    <div className="tree" ref={host} tabIndex={0} onKeyDown={onKeyDown}>
      <div className="tree-root">{props.rootName}</div>
      <div className="tree-body">
        {rows.map((row) => (
          <div
            key={row.path}
            className={row.path === selected ? "tree-row selected" : row.path === props.path ? "tree-row open" : "tree-row"}
            style={{ paddingLeft: 8 + row.depth * 14 }}
            onMouseDown={(e) => (e.preventDefault(), host.current?.focus())}
            onClick={() => activate(row)}
          >
            {row.dir ? (
              expanded.has(row.path) ? <ChevronDown size={13} /> : <ChevronRight size={13} />
            ) : (
              <span className="tree-indent" />
            )}
            {row.dir ? <Folder size={13} /> : <File size={13} />}
            <span className="tree-name">{row.name}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
