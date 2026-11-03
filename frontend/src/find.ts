import type { EditorState } from "@codemirror/state";
import { EditorView, type Panel, type ViewUpdate } from "@codemirror/view";
import { SearchQuery, closeSearchPanel, findNext, findPrevious, getSearchQuery, setSearchQuery } from "@codemirror/search";
import { altLabel } from "./keys";

type Flag = "caseSensitive" | "regexp" | "wholeWord";

// maxCount bounds match counting in very large files.
const maxCount = 9999;

// findPanel is the find bar of the code viewer: the query, the match count,
// match case / regexp / whole word toggles and next / previous. It replaces
// CodeMirror's default panel; replacing text is left out (read-mostly).
export function findPanel(view: EditorView): Panel {
  const dom = el("div", "find");
  const input = el("input", "find-input") as HTMLInputElement;
  input.placeholder = "Find in file";
  input.spellcheck = false;
  input.setAttribute("main-field", "true");
  const count = el("span", "find-count");

  const query = () => getSearchQuery(view.state);
  const commit = (change: Partial<Record<Flag, boolean>> & { search?: string }) => {
    const q = query();
    const next = new SearchQuery({ search: q.search, caseSensitive: q.caseSensitive, regexp: q.regexp, wholeWord: q.wholeWord, ...change });
    if (next.eq(q)) return;
    view.dispatch({ effects: setSearchQuery.of(next) });
    jumpFrom(view, next, view.state.selection.main.from);
  };
  // findNext and findPrevious select the whole query field; keep the caret
  // at its end instead, so typing goes on.
  const step = (command: (v: EditorView) => boolean) => {
    command(view);
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  };
  const flip = (flag: Flag) => commit({ [flag]: !query()[flag] });

  const toggles: [Flag, HTMLButtonElement][] = [
    ["caseSensitive", button("Aa", `Match case (${altLabel}C)`, () => flip("caseSensitive"))],
    ["wholeWord", button("W", `Words (${altLabel}W)`, () => flip("wholeWord"))],
    ["regexp", button(".*", `Regex (${altLabel}R)`, () => flip("regexp"))],
  ];
  dom.append(
    input,
    count,
    ...toggles.map(([, b]) => b),
    button("↑", "Previous (Shift+Enter)", () => step(findPrevious)),
    button("↓", "Next (Enter)", () => step(findNext)),
    button("×", "Close (Esc)", () => (closeSearchPanel(view), view.focus())),
  );

  input.addEventListener("input", () => commit({ search: input.value }));
  input.addEventListener("keydown", (e) => {
    const flag: Flag | null = !e.altKey ? null : e.code === "KeyC" ? "caseSensitive" : e.code === "KeyW" ? "wholeWord" : e.code === "KeyR" ? "regexp" : null;
    if (e.key === "Enter") step(e.shiftKey ? findPrevious : findNext);
    else if (e.key === "Escape") (closeSearchPanel(view), view.focus());
    else if (flag) flip(flag);
    else return;
    e.preventDefault();
    e.stopPropagation();
  });

  const sync = (state: EditorState) => {
    const q = getSearchQuery(state);
    if (input.value !== q.search) input.value = q.search;
    for (const [flag, b] of toggles) b.classList.toggle("active", q[flag]);
    input.classList.toggle("invalid", !!q.search && !q.valid);
    count.textContent = countText(state, q);
  };

  return {
    dom,
    top: true,
    mount: () => (sync(view.state), input.select()),
    update: (u: ViewUpdate) => {
      if (u.docChanged || u.selectionSet || u.transactions.some((t) => t.effects.some((e) => e.is(setSearchQuery)))) sync(u.state);
    },
  };
}

// jumpFrom selects the first match of q at or after pos, wrapping around,
// without touching the query field (search as you type).
function jumpFrom(view: EditorView, q: SearchQuery, pos: number) {
  if (!q.search || !q.valid) return;
  let m = q.getCursor(view.state, pos).next();
  if (m.done) m = q.getCursor(view.state, 0).next();
  if (m.done) return;
  const { from, to } = m.value;
  view.dispatch({
    selection: { anchor: from, head: to },
    effects: EditorView.scrollIntoView(from, { y: "center" }),
    userEvent: "select.search",
  });
}

// countText is "3/12" when a match is selected, "12" otherwise.
function countText(state: EditorState, q: SearchQuery): string {
  if (!q.search || !q.valid) return "";
  const sel = state.selection.main;
  let n = 0;
  let current = 0;
  const cursor = q.getCursor(state);
  for (let m = cursor.next(); !m.done && n < maxCount; m = cursor.next()) {
    n++;
    if (m.value.from === sel.from && m.value.to === sel.to) current = n;
  }
  if (n === 0) return "No results";
  const total = n >= maxCount ? `${maxCount}+` : `${n}`;
  return current ? `${current}/${total}` : total;
}

function el(tag: string, className: string): HTMLElement {
  const e = document.createElement(tag);
  e.className = className;
  return e;
}

function button(label: string, title: string, run: () => void): HTMLButtonElement {
  const b = el("button", "find-button") as HTMLButtonElement;
  b.type = "button";
  b.textContent = label;
  b.title = title;
  b.addEventListener("mousedown", (e) => e.preventDefault());
  b.addEventListener("click", run);
  return b;
}
