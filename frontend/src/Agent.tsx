import { useEffect, useRef, useState, type ReactNode, type RefObject } from "react";
import { ChevronDown, ChevronRight, LoaderCircle } from "lucide-react";
import type { AgentEvent, Hit, Turn } from "./api";

export type Step = { tool: string; input: string; hits: Hit[]; pending: boolean } | { note: string };

export interface Run {
  id: number;
  // thread holds the earlier turns this run follows up on.
  thread: Turn[];
  question: string;
  steps: Step[];
  answer: string;
  status: "running" | "done" | "error";
  error?: string;
  // started is when the run began, for the timer.
  started: number;
  // phase is what the agent is doing right now.
  phase: string;
  // thinking is the reasoning of the current model turn, where shared.
  thinking: string;
}

export function newRun(question: string): Run {
  const now = Date.now();
  return { id: 0, thread: [], question, steps: [], answer: "", status: "running", started: now, phase: "Starting", thinking: "" };
}

// followUp starts a run that continues run's thread with question.
export function followUp(run: Run, question: string): Run {
  const answer = run.answer.trim() || run.error || "No answer.";
  return { ...newRun(question), thread: [...run.thread, { question: run.question, answer }] };
}

// applyEvent folds an agent event into a run. Text streamed before a tool
// call was a progress note, not the answer.
export function applyEvent(run: Run, e: AgentEvent): Run {
  switch (e.kind) {
    case "turn":
      return { ...run, phase: "Thinking", thinking: "" };
    case "thinking":
      return { ...run, thinking: (run.thinking + (e.text ?? "")).slice(-600) };
    case "call": {
      const steps = run.answer.trim() ? [...run.steps, { note: run.answer.trim() }] : run.steps;
      const step = { tool: e.tool ?? "", input: e.input ?? "", hits: [], pending: true };
      return { ...run, answer: "", steps: [...steps, step], phase: `Searching ${stepLabel(step)}` };
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
      return { ...run, answer: run.answer + (e.text ?? ""), phase: run.answer ? run.phase : "Answering" };
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

export interface AgentAnswerProps {
  run: Run;
  onOpen: (root: string, path: string, line: number) => void;
  // onFollowUp, when set, shows a field for a follow-up once the run ends.
  onFollowUp?: (question: string) => void;
  followUpRef?: RefObject<HTMLInputElement>;
  // onFollowUpEscape leaves the follow-up field.
  onFollowUpEscape?: () => void;
}

// AgentAnswer shows the thread's earlier turns, the run's status and the
// answer, with [[root/path:line]] citations rendered as links.
export function AgentAnswer(props: AgentAnswerProps) {
  const { run } = props;
  const running = run.status === "running";
  // Re-render every second while running, for the phase timer.
  const [, tick] = useState(0);
  useEffect(() => {
    if (!running) return;
    const timer = setInterval(() => tick((t) => t + 1), 1000);
    return () => clearInterval(timer);
  }, [running]);
  // Tool calls and reasoning stream fast; the status changes at a readable pace.
  const label = useSteady(run.phase, 800);
  const thought = useSteady(lastSentence(run.thinking), 800);
  const seconds = Math.floor((Date.now() - run.started) / 1000);
  return (
    <div className="agent-answer">
      {run.thread.map((turn, i) => (
        <EarlierTurn key={i} turn={turn} onOpen={props.onOpen} />
      ))}
      {run.thread.length > 0 && <div className="agent-question">{run.question}</div>}
      {running && (
        <div className="agent-status">
          <LoaderCircle size={14} className="spin" />
          <span key={label} className="agent-phase fade-in">
            {label}
          </span>
          <span className="agent-timer">{seconds}s</span>
        </div>
      )}
      {running && thought && !run.answer && (
        <div key={thought} className="agent-thinking fade-in">
          {thought}
        </div>
      )}
      {run.answer && <Markdown text={run.answer} onOpen={props.onOpen} />}
      {run.status === "error" && <div className="error">{run.error}</div>}
      {props.onFollowUp && !running && (
        <input
          ref={props.followUpRef}
          className="follow-up"
          placeholder="Ask a follow-up…"
          // After a follow-up, the next one can be typed right away.
          autoFocus={run.thread.length > 0}
          spellCheck={false}
          onKeyDown={(e) => {
            if (e.key === "Enter" && e.currentTarget.value.trim()) {
              props.onFollowUp!(e.currentTarget.value.trim());
              e.currentTarget.value = "";
            } else if (e.key === "Escape" || (e.key === "Tab" && e.shiftKey)) {
              props.onFollowUpEscape?.();
            } else {
              return;
            }
            e.preventDefault();
            e.stopPropagation();
          }}
        />
      )}
    </div>
  );
}

// EarlierTurn is a collapsed earlier question of the thread; it expands to
// its answer.
function EarlierTurn(props: { turn: Turn; onOpen: (root: string, path: string, line: number) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="agent-turn">
      <button className="agent-turn-question" onClick={() => setOpen(!open)}>
        {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        <span>{props.turn.question}</span>
      </button>
      {open && <Markdown text={props.turn.answer} onOpen={props.onOpen} />}
    </div>
  );
}

// lastSentence is the last finished sentence of the reasoning so far, so
// the shown thought does not change with every streamed word.
function lastSentence(text: string): string {
  const sentences = text.replace(/\s+/g, " ").match(/[^.!?]+[.!?]+(?=\s|$)/g);
  const last = sentences?.[sentences.length - 1]?.trim() ?? "";
  return last.length > 220 ? "…" + last.slice(-220) : last;
}

// useSteady follows value but changes at most once per interval: a fast
// series of updates shows its latest value at the end of each interval.
function useSteady<T>(value: T, ms: number): T {
  const [shown, setShown] = useState(value);
  const changed = useRef(0);
  useEffect(() => {
    const wait = Math.max(0, changed.current + ms - Date.now());
    const timer = setTimeout(() => {
      changed.current = Date.now();
      setShown(value);
    }, wait);
    return () => clearTimeout(timer);
  }, [value, ms]);
  return shown;
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
