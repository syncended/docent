import { useEffect, useState, type ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import type { AgentEvent, Hit } from "./api";

export type Step = { tool: string; input: string; hits: Hit[]; pending: boolean } | { note: string };

// Phase is what the agent is doing right now, shown with a timer.
export interface Phase {
  label: string;
  since: number;
}

export interface Run {
  id: number;
  question: string;
  steps: Step[];
  answer: string;
  status: "running" | "done" | "error";
  error?: string;
  phase: Phase;
  // thinking is the reasoning of the current model turn, where shared.
  thinking: string;
}

export function newRun(question: string): Run {
  return { id: 0, question, steps: [], answer: "", status: "running", phase: { label: "Starting", since: Date.now() }, thinking: "" };
}

// applyEvent folds an agent event into a run. Text streamed before a tool
// call was a progress note, not the answer.
export function applyEvent(run: Run, e: AgentEvent): Run {
  const now = Date.now();
  switch (e.kind) {
    case "turn":
      return { ...run, phase: { label: "Thinking", since: now }, thinking: "" };
    case "thinking":
      return { ...run, thinking: (run.thinking + (e.text ?? "")).slice(-600) };
    case "call": {
      const steps = run.answer.trim() ? [...run.steps, { note: run.answer.trim() }] : run.steps;
      const step = { tool: e.tool ?? "", input: e.input ?? "", hits: [], pending: true };
      return { ...run, answer: "", steps: [...steps, step], phase: { label: `Searching ${stepLabel(step)}`, since: now } };
    }
    case "step": {
      const steps = [...run.steps];
      const i = steps.findLastIndex((s) => "tool" in s && s.pending && s.tool === e.tool);
      const done = { tool: e.tool ?? "", input: e.input ?? "", hits: e.hits ?? [], pending: false };
      if (i >= 0) steps[i] = done;
      else steps.push(done);
      return { ...run, steps };
    }
    case "text":
      return { ...run, answer: run.answer + (e.text ?? ""), phase: run.answer ? run.phase : { label: "Answering", since: now } };
    case "done":
      return { ...run, status: "done" };
    case "error":
      return { ...run, status: "error", error: e.text };
  }
}

const toolLabels: Record<string, string> = {
  search_types: "Types",
  search_symbols: "Symbols",
  search_files: "Files",
  search_text: "Text",
  search_regex: "Regex",
  find_usages: "Usages",
  read_file: "Read",
};

export function stepLabel(step: Step): string {
  if ("note" in step) return step.note;
  const label = `${toolLabels[step.tool] ?? step.tool} · ${step.input}`;
  if (step.pending) return `${label} …`;
  return step.tool === "read_file" ? label : `${label} · ${step.hits.length} found`;
}

// AgentAnswer shows the run's status and the answer, with [[root/path:line]]
// citations rendered as links.
export function AgentAnswer(props: { run: Run; onOpen: (root: string, path: string, line: number) => void }) {
  const { run } = props;
  const running = run.status === "running";
  // Re-render every second while running, for the phase timer.
  const [, tick] = useState(0);
  useEffect(() => {
    if (!running) return;
    const timer = setInterval(() => tick((t) => t + 1), 1000);
    return () => clearInterval(timer);
  }, [running]);
  const seconds = Math.floor((Date.now() - run.phase.since) / 1000);
  return (
    <div className="agent-answer">
      {running && (
        <div className="agent-status">
          <LoaderCircle size={14} className="spin" />
          <span className="agent-phase">{run.phase.label}</span>
          <span className="agent-timer">{seconds}s</span>
        </div>
      )}
      {running && run.thinking && !run.answer && <div className="agent-thinking">{tail(run.thinking)}</div>}
      {run.answer && <Markdown text={run.answer} onOpen={props.onOpen} />}
      {run.status === "error" && <div className="error">{run.error}</div>}
    </div>
  );
}

// tail is the end of the reasoning so far, starting at a word boundary.
function tail(text: string): string {
  const t = text.trim().replace(/\s+/g, " ");
  if (t.length <= 220) return t;
  const cut = t.slice(-220);
  return "…" + cut.slice(cut.indexOf(" ") + 1);
}

// Markdown renders the small subset agents use: paragraphs, lists, code
// blocks, inline code, bold and citations.
function Markdown(props: { text: string; onOpen: (root: string, path: string, line: number) => void }) {
  const blocks: ReactNode[] = [];
  const parts = props.text.split(/^```.*$/m);
  parts.forEach((part, i) => {
    if (i % 2 === 1) {
      blocks.push(<pre key={i}>{part.replace(/^\n|\n$/g, "")}</pre>);
      return;
    }
    let list: ReactNode[] = [];
    const flush = () => {
      if (list.length) blocks.push(<ul key={`${i}-${blocks.length}`}>{list}</ul>);
      list = [];
    };
    part.split("\n").forEach((line, j) => {
      const item = line.match(/^\s*(?:[-*]|\d+\.)\s+(.*)$/);
      if (item) {
        list.push(<li key={j}>{inline(item[1], props.onOpen)}</li>);
        return;
      }
      flush();
      if (line.trim()) blocks.push(<p key={`${i}-${j}`}>{inline(line.replace(/^#+\s*/, ""), props.onOpen)}</p>);
    });
    flush();
  });
  return <div className="markdown">{blocks}</div>;
}

function inline(text: string, onOpen: (root: string, path: string, line: number) => void): ReactNode[] {
  return text.split(/(\[\[[^\]]+\]\]|`[^`]+`|\*\*[^*]+\*\*)/).map((tok, i) => {
    if (tok.startsWith("[[") && tok.endsWith("]]")) {
      const ref = tok.slice(2, -2);
      const slash = ref.indexOf("/");
      const m = ref.slice(slash + 1).match(/^(.*?)(?::(\d+)(?:-\d+)?)?$/);
      if (slash > 0 && m) {
        const path = m[1];
        const line = Number(m[2] ?? 1);
        return (
          <button key={i} className="cite" onClick={() => onOpen(ref.slice(0, slash), path, line)}>
            {path.slice(path.lastIndexOf("/") + 1)}
            {m[2] ? `:${m[2]}` : ""}
          </button>
        );
      }
    }
    if (tok.startsWith("`") && tok.endsWith("`") && tok.length > 1) return <code key={i}>{tok.slice(1, -1)}</code>;
    if (tok.startsWith("**") && tok.endsWith("**")) return <b key={i}>{tok.slice(2, -2)}</b>;
    return tok;
  });
}
