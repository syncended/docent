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

// names are the languages shown in the status bar, by extension; a file
// of another kind is named after its extension.
const names: Record<string, string> = {
  ts: "TypeScript", mts: "TypeScript", cts: "TypeScript", tsx: "TSX",
  js: "JavaScript", mjs: "JavaScript", cjs: "JavaScript", jsx: "JSX",
  java: "Java", kt: "Kotlin", kts: "Kotlin", c: "C", cc: "C++", cpp: "C++", cxx: "C++", h: "C/C++ header", hh: "C++", hpp: "C++",
  go: "Go", rs: "Rust", swift: "Swift", cs: "C#", scala: "Scala", sc: "Scala",
  rb: "Ruby", rake: "Ruby", gemspec: "Ruby", py: "Python", pyi: "Python", lua: "Lua", groovy: "Groovy", gradle: "Gradle",
  elm: "Elm", ml: "OCaml", mli: "OCaml", php: "PHP", ex: "Elixir", exs: "Elixir", proto: "Protobuf",
  md: "Markdown", mdx: "Markdown", json: "JSON", yaml: "YAML", yml: "YAML", toml: "TOML", sql: "SQL",
  css: "CSS", html: "HTML", svelte: "Svelte", vue: "Vue", sh: "Shell", bash: "Shell", tf: "Terraform", tfvars: "Terraform", hcl: "HCL",
};

// languageName returns the language of a file path for people.
export function languageName(path: string): string {
  const file = path.slice(path.lastIndexOf("/") + 1);
  const dot = file.lastIndexOf(".");
  if (dot <= 0) return "Plain text";
  const ext = file.slice(dot + 1).toLowerCase();
  return names[ext] ?? ext.toUpperCase();
}

// languageFor returns the syntax highlighting for a file path.
export function languageFor(path: string): Extension {
  const ext = path.slice(path.lastIndexOf(".") + 1).toLowerCase();
  return byExtension[ext]?.() ?? [];
}
