package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/syncended/docent/core/code"
	"github.com/syncended/docent/core/config"
	"github.com/syncended/docent/core/secrets"
	"github.com/syncended/docent/core/syntax"
)

// apiMiddleware serves /api/* to the frontend; everything else goes to the
// asset handler.
func apiMiddleware(next http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", handleStatus)
	mux.HandleFunc("GET /api/search", handleSearch)
	mux.HandleFunc("GET /api/file", handleReadFile)
	mux.HandleFunc("PUT /api/file", handleWriteFile)
	mux.HandleFunc("GET /api/definitions", handleDefinitions)
	mux.HandleFunc("GET /api/usages", handleUsages)
	mux.HandleFunc("POST /api/hide", handleHide)
	mux.HandleFunc("POST /api/layout", handleLayout)
	mux.HandleFunc("POST /api/reindex", handleReindex)
	mux.HandleFunc("GET /api/settings", handleGetSettings)
	mux.HandleFunc("PUT /api/settings", handlePutSettings)
	mux.HandleFunc("POST /api/pick-folder", handlePickFolder)
	mux.HandleFunc("POST /api/highlight", handleHighlight)
	mux.HandleFunc("POST /api/agent/ask", handleAgentAsk)
	mux.HandleFunc("POST /api/agent/stop", handleAgentStop)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			mux.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type status struct {
	ConfigPath string          `json:"configPath"`
	Hotkey     string          `json:"hotkey"`
	Indexing   bool            `json:"indexing"`
	Roots      []code.RootInfo `json:"roots"`
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, status{ConfigPath: configPath, Hotkey: hotkey, Indexing: indexing.Load(), Roots: code.Roots()})
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := intParam(r, "limit", 50)
	switch code.Tab(r.URL.Query().Get("tab")) {
	case code.TabAll:
		writeJSON(w, map[string]any{"sections": code.SearchAll(r.Context(), q, min(limit, 8))})
	case code.TabTypes:
		writeJSON(w, map[string]any{"hits": code.SearchTypes(q, limit)})
	case code.TabSymbols:
		writeJSON(w, map[string]any{"hits": code.SearchSymbols(q, limit)})
	case code.TabFiles:
		writeJSON(w, map[string]any{"hits": code.SearchFiles(q, limit)})
	case code.TabText:
		hits, err := textSearch(r)(r.Context(), q, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, map[string]any{"hits": hits})
	default:
		writeError(w, http.StatusBadRequest, errString("unknown tab"))
	}
}

// textSearch picks the text search for the Regex and Match case toggles.
func textSearch(r *http.Request) func(context.Context, string, int) ([]code.Hit, error) {
	regex, matchCase := r.URL.Query().Get("regex") == "1", r.URL.Query().Get("case") == "1"
	switch {
	case regex && matchCase:
		return code.SearchRegexMatchCase
	case regex:
		return code.SearchRegex
	case matchCase:
		return code.SearchTextMatchCase
	default:
		return code.SearchText
	}
}

// handleHighlight highlights the request body as the file at ?path, for
// languages the frontend cannot highlight well itself.
func handleHighlight(w http.ResponseWriter, r *http.Request) {
	src, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	spans, err := syntax.Highlight(r.Context(), r.URL.Query().Get("path"), src)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]any{"spans": spans})
}

func handleAgentAsk(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Question) == "" {
		writeError(w, http.StatusBadRequest, errString("question is required"))
		return
	}
	run, err := startAgent(in.Question)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]int64{"run": run})
}

func handleAgentStop(w http.ResponseWriter, r *http.Request) {
	stopAgent()
	writeJSON(w, map[string]bool{"ok": true})
}

func handleReadFile(w http.ResponseWriter, r *http.Request) {
	content, err := code.ReadFile(r.URL.Query().Get("root"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, map[string]string{"content": content})
}

func handleWriteFile(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := code.WriteFile(r.URL.Query().Get("root"), r.URL.Query().Get("path"), string(body)); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func handleDefinitions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"hits": code.Definitions(r.URL.Query().Get("ident"), intParam(r, "limit", 20))})
}

func handleUsages(w http.ResponseWriter, r *http.Request) {
	hits, err := code.Usages(r.Context(), r.URL.Query().Get("ident"), intParam(r, "limit", 100))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]any{"hits": hits})
}

func handleHide(w http.ResponseWriter, r *http.Request) {
	hidePalette()
	writeJSON(w, map[string]bool{"ok": true})
}

func handleLayout(w http.ResponseWriter, r *http.Request) {
	setLayout(r.URL.Query().Get("name"))
	writeJSON(w, map[string]bool{"ok": true})
}

func handleReindex(w http.ResponseWriter, r *http.Request) {
	go reindex()
	writeJSON(w, map[string]bool{"ok": true})
}

// settings is what the settings screen edits. The API key is write-only:
// reads report only whether one is stored.
type settings struct {
	ConfigPath string         `json:"configPath"`
	Hotkey     string         `json:"hotkey"`
	Roots      []config.Root  `json:"roots"`
	Agent      config.Agent   `json:"agent"`
	Palette    config.Palette `json:"palette"`
	HasKey     bool           `json:"hasKey"`
	// APIKey, if not empty, replaces the stored key; ClearKey removes it.
	APIKey   string `json:"apiKey,omitempty"`
	ClearKey bool   `json:"clearKey,omitempty"`
}

func handleGetSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load(configPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, currentSettings(cfg))
}

func currentSettings(cfg *config.Config) settings {
	key, _ := secrets.APIKey(cfg.Agent.Provider)
	roots := cfg.Roots
	if roots == nil {
		roots = []config.Root{}
	}
	return settings{ConfigPath: configPath, Hotkey: hotkey, Roots: roots, Agent: cfg.Agent, Palette: cfg.Palette, HasKey: key != ""}
}

func handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var in settings
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg := &config.Config{Agent: in.Agent, Palette: in.Palette}
	for _, root := range in.Roots {
		root.Path = strings.TrimSpace(root.Path)
		if root.Path == "" {
			continue
		}
		if strings.HasPrefix(root.Path, "~/") {
			home, _ := os.UserHomeDir()
			root.Path = filepath.Join(home, root.Path[2:])
		}
		if info, err := os.Stat(root.Path); err != nil || !info.IsDir() {
			writeError(w, http.StatusBadRequest, fmt.Errorf("%s is not a directory", root.Path))
			return
		}
		cfg.Roots = append(cfg.Roots, root)
	}
	if provider := cfg.Agent.Provider; provider != "" {
		var err error
		switch {
		case in.APIKey != "":
			err = secrets.SetAPIKey(provider, strings.TrimSpace(in.APIKey))
		case in.ClearKey:
			err = secrets.DeleteAPIKey(provider)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("credential store: %w", err))
			return
		}
	}
	if err := applyConfig(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, currentSettings(cfg))
}

func handlePickFolder(w http.ResponseWriter, r *http.Request) {
	path, err := pickFolder()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, map[string]string{"path": path})
}

func intParam(r *http.Request, name string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && n > 0 {
		return n
	}
	return def
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

type errString string

func (e errString) Error() string { return string(e) }
