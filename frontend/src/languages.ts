import type { Extension } from "@codemirror/state";
import { StreamLanguage } from "@codemirror/language";
import { javascript } from "@codemirror/lang-javascript";
import { java } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { go } from "@codemirror/lang-go";
import { markdown } from "@codemirror/lang-markdown";
import { json } from "@codemirror/lang-json";
import { python } from "@codemirror/lang-python";
import { yaml } from "@codemirror/lang-yaml";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { sql } from "@codemirror/legacy-modes/mode/sql";
import { toml } from "@codemirror/legacy-modes/mode/toml";

const byExtension: Record<string, () => Extension> = {
  ts: () => javascript({ typescript: true }),
  mts: () => javascript({ typescript: true }),
  cts: () => javascript({ typescript: true }),
  tsx: () => javascript({ typescript: true, jsx: true }),
  js: () => javascript(),
  mjs: () => javascript(),
  cjs: () => javascript(),
  jsx: () => javascript({ jsx: true }),
  java: () => java(),
  c: () => cpp(),
  cc: () => cpp(),
  cpp: () => cpp(),
  cxx: () => cpp(),
  h: () => cpp(),
  hh: () => cpp(),
  hpp: () => cpp(),
  go: () => go(),
  md: () => markdown(),
  mdx: () => markdown(),
  json: () => json(),
  py: () => python(),
  yaml: () => yaml(),
  yml: () => yaml(),
  sh: () => StreamLanguage.define(shell),
  bash: () => StreamLanguage.define(shell),
  sql: () => StreamLanguage.define(sql({})),
  toml: () => StreamLanguage.define(toml),
};

// languageFor returns the syntax highlighting for a file path.
export function languageFor(path: string): Extension {
  const ext = path.slice(path.lastIndexOf(".") + 1).toLowerCase();
  return byExtension[ext]?.() ?? [];
}
