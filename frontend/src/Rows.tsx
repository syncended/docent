import { useEffect, useRef } from "react";
import type { Hit } from "./api";

// Result rows shared by the palette, the viewer's search context and the
// overview window's results panel.

export type Row = { type: "header"; label: string } | { type: "note"; text: string } | { type: "hit"; hit: Hit } | { type: "agent"; query: string };

export function RowView(props: { row: Row; selected: boolean; onSelect: () => void; onOpen: () => void }) {
  const { row } = props;
  const el = useRef<HTMLDivElement>(null);
  // Scroll only when the selection moves: rows re-render while an agent
  // streams, and following the row on every render drags the list along.
  useEffect(() => {
    if (props.selected) el.current?.scrollIntoView({ block: "nearest" });
  }, [props.selected]);
  if (row.type === "header") return <div className="row-header">{row.label}</div>;
  if (row.type === "note") return <div className="row-note">{row.text}</div>;
  const cls = props.selected ? "row selected" : "row";
  if (row.type === "agent") {
    return (
      <div className={cls} onClick={props.onSelect} onDoubleClick={props.onOpen}>
        <div className="row-main">
          <span className="kind agent">?</span>
          <span className="title">Ask agent: {row.query}</span>
        </div>
      </div>
    );
  }
  const { hit } = row;
  return (
    <div className={cls} onClick={props.onSelect} onDoubleClick={props.onOpen} ref={el}>
      <div className="row-main">
        <span className={`kind k-${hit.kind ?? hit.tab}`}>{kindLabel(hit.kind ?? hit.tab)}</span>
        <span className="title">
          <Highlighted text={hit.title} positions={hit.titleMatches} />
        </span>
        {hit.container && hit.tab !== "files" && hit.tab !== "text" && <span className="container">{hit.container}</span>}
        <span className="location">{location(hit)}</span>
      </div>
      {hit.snippet && <SnippetLine hit={hit} />}
    </div>
  );
}

// location is shown at the right of a row: where the hit is, without
// repeating the title.
function location(hit: Hit): string {
  const line = hit.line > 0 ? `:${hit.line}` : "";
  const file = hit.path.slice(hit.path.lastIndexOf("/") + 1);
  switch (hit.tab) {
    case "files":
      return `${hit.container ?? ""} · ${hit.rootName}`;
    case "text":
      return `${hit.path}${line} · ${hit.rootName}`;
    default:
      return `${file}${line} · ${hit.rootName}`;
  }
}

function Highlighted(props: { text: string; positions?: number[] }) {
  const marks = new Set(props.positions ?? []);
  return (
    <>
      {Array.from(props.text).map((ch, i) => (marks.has(i) ? <mark key={i}>{ch}</mark> : ch))}
    </>
  );
}

// SnippetLine shows the matched line of a hit, with matches highlighted.
function SnippetLine({ hit }: { hit: Hit }) {
  const sn = hit.snippet!;
  const idx = Math.max(0, Math.min(hit.line - sn.startLine, sn.lines.length - 1));
  const line = sn.lines[idx] ?? "";
  const indent = line.length - line.trimStart().length;
  const matches = (sn.matches ?? []).filter((m) => m.line === idx);
  const parts: JSX.Element[] = [];
  let pos = indent;
  matches.forEach((m, i) => {
    const start = Math.max(m.start, pos);
    if (start > pos) parts.push(<span key={`t${i}`}>{line.slice(pos, start)}</span>);
    parts.push(<mark key={`m${i}`}>{line.slice(start, m.end)}</mark>);
    pos = Math.max(pos, m.end);
  });
  parts.push(<span key="rest">{line.slice(pos)}</span>);
  return <div className="snippet">{parts}</div>;
}

function kindLabel(kind: string): string {
  const labels: Record<string, string> = {
    class: "C", interface: "I", enum: "E", struct: "S", union: "U", object: "O", message: "M", service: "Sv",
    type: "T", namespace: "N", function: "ƒ", method: "m", rpc: "R", field: "f", property: "p",
    constant: "c", variable: "v", enum_value: "e", files: "F", text: "T",
  };
  return labels[kind] ?? kind[0];
}

// crumbs splits a file path into its folders, each with its path in the
// root ("" for the root), and the file name last.
export function crumbs(rootName: string, path: string): { name: string; path: string }[] {
  const parts = path.split("/");
  return [{ name: rootName, path: "" }, ...parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }))];
}

export function sameHit(a: Hit, b: Hit): boolean {
  return a.root === b.root && a.path === b.path && a.line === b.line;
}
