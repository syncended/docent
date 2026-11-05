import type { Extension } from "@codemirror/state";
import { StreamLanguage } from "@codemirror/language";
import { javascript } from "@codemirror/lang-javascript";
import { java } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { css } from "@codemirror/lang-css";
import { html } from "@codemirror/lang-html";
import { go } from "@codemirror/lang-go";
import { markdown } from "@codemirror/lang-markdown";
import { json } from "@codemirror/lang-json";
import { python } from "@codemirror/lang-python";
import { yaml } from "@codemirror/lang-yaml";
import { csharp, scala } from "@codemirror/legacy-modes/mode/clike";
import { elm } from "@codemirror/legacy-modes/mode/elm";
import { groovy } from "@codemirror/legacy-modes/mode/groovy";
import { lua } from "@codemirror/legacy-modes/mode/lua";
import { oCaml } from "@codemirror/legacy-modes/mode/mllike";
import { ruby } from "@codemirror/legacy-modes/mode/ruby";
import { rust } from "@codemirror/legacy-modes/mode/rust";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { swift } from "@codemirror/legacy-modes/mode/swift";
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
  pyi: () => python(),
  rs: () => StreamLanguage.define(rust),
  swift: () => StreamLanguage.define(swift),
  cs: () => StreamLanguage.define(csharp),
  scala: () => StreamLanguage.define(scala),
  sc: () => StreamLanguage.define(scala),
  rb: () => StreamLanguage.define(ruby),
  rake: () => StreamLanguage.define(ruby),
  gemspec: () => StreamLanguage.define(ruby),
  lua: () => StreamLanguage.define(lua),
  groovy: () => StreamLanguage.define(groovy),
  gradle: () => StreamLanguage.define(groovy),
  elm: () => StreamLanguage.define(elm),
  ml: () => StreamLanguage.define(oCaml),
  mli: () => StreamLanguage.define(oCaml),
  css: () => css(),
  html: () => html(),
  svelte: () => html(),
  vue: () => html(),
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
