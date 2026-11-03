// Package agent is Docent's built-in code search agent: a tool-calling loop
// over a model provider, using the code package as its only (read-only) tools.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/syncended/docent/core/code"
)

// Event reports the progress of a run to the UI.
type Event struct {
	// Kind is one of:
	//   "turn"     the model started working on its next move
	//   "thinking" a piece of the model's reasoning, where the provider shares it
	//   "call"     the model called a tool (Tool, Input); its result follows
	//   "step"     the tool call finished (Tool, Input, Hits)
	//   "text"     a piece of answer text
	//   "done", "error"
	Kind  string     `json:"kind"`
	Tool  string     `json:"tool,omitempty"`
	Input string     `json:"input,omitempty"`
	Text  string     `json:"text,omitempty"`
	Hits  []code.Hit `json:"hits,omitempty"`
}

// Config selects the provider and model.
type Config struct {
	// Provider is "anthropic" or "openai" (any OpenAI-compatible API).
	Provider string
	Model    string
	BaseURL  string
	APIKey   string
}

// maxSteps bounds the number of model turns in one run.
const maxSteps = 20

// ErrNoKey is returned when no API key is configured.
var ErrNoKey = errors.New("no API key configured: open Settings (⌘,) and paste one")

// Ask answers question by searching the indexed code, reporting progress
// through emit. It returns when the answer is complete or ctx is cancelled.
func Ask(ctx context.Context, cfg Config, question string, emit func(Event)) error {
	if cfg.APIKey == "" {
		return ErrNoKey
	}
	switch cfg.Provider {
	case "anthropic":
		return askAnthropic(ctx, cfg, question, emit)
	case "openai":
		return askOpenAI(ctx, cfg, question, emit)
	default:
		return fmt.Errorf("unknown agent provider %q", cfg.Provider)
	}
}

const systemPrompt = `You are the search agent of Docent, a read-only code browser. A developer asks where or how something is implemented in their local code; you find it and explain briefly.

You can only read code, using the provided tools. You cannot modify files or run commands.

How to search:
- Use search_types and search_symbols when the question names or implies a concept: for "where is the cart discount calculated", try names like Discount or CartDiscount. These searches are fuzzy and fast.
- Use search_text or search_regex for string literals, config keys, log messages, and when names are unknown. Use search_files for file names.
- Read only the relevant line ranges with read_file, not whole large files.
- Stop as soon as you can answer. A handful of tool calls is usually enough.

How to answer:
- Be concise: a few sentences or a short list.
- Cite every code location you mention as [[root/path:line]], copied exactly from tool results.
- Answer in the language of the question.
- If you could not find it, say so and say where you looked.`

// tool is a provider-neutral tool definition.
type tool struct {
	name        string
	description string
	// properties is the JSON schema of each parameter; all listed in
	// required must be present.
	properties map[string]any
	required   []string
	run        func(ctx context.Context, input map[string]any) (result string, hits []code.Hit, err error)
}

func stringParam(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func intParam(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

const listLimit = 15

var tools = []tool{
	{
		name:        "search_types",
		description: "Find classes, interfaces, enums, structs, proto messages and services by fuzzy name (camel-hump abbreviations work: OrSt finds OrderStatus). Use Parent.Name to match nested types.",
		properties:  map[string]any{"query": stringParam("Type name or part of it")},
		required:    []string{"query"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits := code.SearchTypes(str(in, "query"), listLimit)
			return formatHits(hits), hits, nil
		},
	},
	{
		name:        "search_symbols",
		description: "Find any declaration (function, method, field, constant, enum value, type) by fuzzy name. Use Parent.name to match members of a type.",
		properties:  map[string]any{"query": stringParam("Symbol name or part of it")},
		required:    []string{"query"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits := code.SearchSymbols(str(in, "query"), listLimit)
			return formatHits(hits), hits, nil
		},
	},
	{
		name:        "search_files",
		description: "Find files by fuzzy name, or by path when the query contains a slash.",
		properties:  map[string]any{"query": stringParam("File name or path fragment")},
		required:    []string{"query"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits := code.SearchFiles(str(in, "query"), listLimit)
			return formatHits(hits), hits, nil
		},
	},
	{
		name:        "search_text",
		description: "Find lines containing a literal string, ignoring case.",
		properties:  map[string]any{"query": stringParam("Literal text, at least 3 characters")},
		required:    []string{"query"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits, err := code.SearchText(ctx, str(in, "query"), listLimit)
			return formatHits(hits), hits, err
		},
	},
	{
		name:        "search_regex",
		description: "Find lines matching a regular expression (RE2 syntax), ignoring case.",
		properties:  map[string]any{"pattern": stringParam("Regular expression")},
		required:    []string{"pattern"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits, err := code.SearchRegex(ctx, str(in, "pattern"), listLimit)
			return formatHits(hits), hits, err
		},
	},
	{
		name:        "find_usages",
		description: "Find lines where an identifier is used as a whole word (case-sensitive).",
		properties:  map[string]any{"identifier": stringParam("Exact identifier")},
		required:    []string{"identifier"},
		run: func(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
			hits, err := code.Usages(ctx, str(in, "identifier"), 30)
			return formatHits(hits), hits, err
		},
	},
	{
		name:        "read_file",
		description: "Read lines of a file. Lines are numbered. Read at most 200 lines at a time.",
		properties: map[string]any{
			"file":       stringParam("File as root/path, copied from a tool result"),
			"start_line": intParam("First line, 1-based"),
			"end_line":   intParam("Last line, inclusive"),
		},
		required: []string{"file", "start_line", "end_line"},
		run:      readFile,
	},
}

const maxReadLines = 200

func readFile(ctx context.Context, in map[string]any) (string, []code.Hit, error) {
	root, path, ok := strings.Cut(str(in, "file"), "/")
	if !ok {
		return "", nil, fmt.Errorf("file must be root/path, as printed in tool results")
	}
	content, err := code.ReadFile(root, path)
	if err != nil {
		return "", nil, err
	}
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	start := max(num(in, "start_line"), 1)
	end := min(num(in, "end_line"), len(lines), start+maxReadLines-1)
	if start > len(lines) {
		return "", nil, fmt.Errorf("%s has %d lines", path, len(lines))
	}
	var b strings.Builder
	for i := start; i <= end; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, lines[i-1])
	}
	if end < len(lines) {
		fmt.Fprintf(&b, "(%d more lines)\n", len(lines)-end)
	}
	hit := code.Hit{Tab: code.TabFiles, Root: root, RootName: rootName(root), Path: path, Line: start, Title: path[strings.LastIndex(path, "/")+1:]}
	return b.String(), []code.Hit{hit}, nil
}

func rootName(id string) string {
	for _, r := range code.Roots() {
		if r.ID == id {
			return r.Name
		}
	}
	return id
}

// formatHits renders hits for the model, one location per line with the
// matched line of code below it.
func formatHits(hits []code.Hit) string {
	if len(hits) == 0 {
		return "No results."
	}
	var b strings.Builder
	for _, h := range hits {
		loc := h.Root + "/" + h.Path
		if h.Line > 0 {
			loc += fmt.Sprintf(":%d", h.Line)
		}
		name := h.Title
		if h.Container != "" && h.Tab != code.TabFiles && h.Tab != code.TabText {
			name = h.Container + "." + name
		}
		if h.Kind != "" {
			name = h.Kind + " " + name
		}
		if h.Tab == code.TabText || h.Tab == code.TabFiles {
			name = ""
		}
		fmt.Fprintf(&b, "%s  %s\n", loc, name)
		if sn := h.Snippet; sn != nil {
			if i := h.Line - sn.StartLine; i >= 0 && i < len(sn.Lines) {
				fmt.Fprintf(&b, "    %s\n", strings.TrimSpace(sn.Lines[i]))
			}
		}
	}
	return b.String()
}

// callTool runs a tool call, reporting it before and after, and returns the
// result for the model.
func callTool(ctx context.Context, name string, rawInput []byte, emit func(Event)) (result string, isError bool) {
	var in map[string]any
	json.Unmarshal(rawInput, &in)
	emit(Event{Kind: "call", Tool: name, Input: summarize(in)})
	result, hits, input, err := runTool(ctx, name, rawInput)
	emit(Event{Kind: "step", Tool: name, Input: input, Hits: hits})
	if err != nil {
		return "Error: " + err.Error(), true
	}
	return result, false
}

// runTool executes a tool call, validating its input against the schema.
func runTool(ctx context.Context, name string, rawInput []byte) (result string, hits []code.Hit, input string, err error) {
	var t *tool
	for i := range tools {
		if tools[i].name == name {
			t = &tools[i]
		}
	}
	if t == nil {
		return "", nil, "", fmt.Errorf("unknown tool %q", name)
	}
	var in map[string]any
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return "", nil, "", fmt.Errorf("invalid JSON input: %w", err)
	}
	for _, r := range t.required {
		if _, ok := in[r]; !ok {
			return "", nil, "", fmt.Errorf("missing required parameter %q", r)
		}
	}
	input = summarize(in)
	result, hits, err = t.run(ctx, in)
	return result, hits, input, err
}

// summarize is a short, human-readable form of a tool input for the UI.
func summarize(in map[string]any) string {
	for _, k := range []string{"query", "pattern", "identifier"} {
		if v := str(in, k); v != "" {
			return v
		}
	}
	if f := str(in, "file"); f != "" {
		_, path, _ := strings.Cut(f, "/")
		return fmt.Sprintf("%s:%d-%d", path, num(in, "start_line"), num(in, "end_line"))
	}
	return ""
}

func str(in map[string]any, k string) string {
	s, _ := in[k].(string)
	return s
}

func num(in map[string]any, k string) int {
	switch v := in[k].(type) {
	case float64:
		return int(v)
	case string:
		var n int
		fmt.Sscan(v, &n)
		return n
	}
	return 0
}
