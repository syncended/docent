package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/syncended/docent/core/code"
	"github.com/syncended/docent/core/config"
)

func setupIndex(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	src := "package shop\n\ntype OrderStatus int\n\nconst StatusDone OrderStatus = 1\n"
	if err := os.MkdirAll(filepath.Join(dir, "repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "repo", "order.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Roots: []config.Root{{Path: filepath.Join(dir, "repo")}}}
	if err := code.Open(cfg, filepath.Join(dir, "cache")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(code.Close)
	if err := code.Reindex(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); !code.TextReady(); {
		if time.Now().After(deadline) {
			t.Fatal("text index did not load")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAskOpenAI(t *testing.T) {
	setupIndex(t)
	root := code.Roots()[0].ID

	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request %s, auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		var req struct {
			Model    string           `json:"model"`
			Messages []map[string]any `json:"messages"`
			Tools    []map[string]any `json:"tools"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "test-model" || len(req.Tools) != len(tools) {
			t.Errorf("model %q, %d tools", req.Model, len(req.Tools))
		}
		turn++
		var msg map[string]any
		switch turn {
		case 1:
			if len(req.Messages) != 4 || req.Messages[1]["content"] != "what is an order?" ||
				req.Messages[2]["role"] != "assistant" || req.Messages[2]["content"] != "A struct." ||
				req.Messages[3]["content"] != "where is the order status?" {
				t.Errorf("thread not passed before the question: %v", req.Messages)
			}
			msg = map[string]any{
				"role": "assistant", "content": nil, "reasoning_content": "thinking",
				"tool_calls": []map[string]any{{
					"id": "call_1", "type": "function",
					"function": map[string]any{"name": "search_types", "arguments": `{"query":"OrdSt"}`},
				}},
			}
		default:
			last := req.Messages[len(req.Messages)-1]
			if last["role"] != "tool" || !strings.Contains(last["content"].(string), root+"/order.go:3") {
				t.Errorf("tool result not passed back: %v", last)
			}
			if req.Messages[len(req.Messages)-2]["reasoning_content"] != "thinking" {
				t.Errorf("assistant message not passed back verbatim: %v", req.Messages[len(req.Messages)-2])
			}
			msg = map[string]any{"role": "assistant", "content": "It is [[" + root + "/order.go:3]]."}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"message": msg}}})
	}))
	defer server.Close()

	var events []Event
	thread := []Turn{{Question: "what is an order?", Answer: "A struct."}}
	err := Ask(context.Background(), Config{Provider: "openai", Model: "test-model", BaseURL: server.URL + "/v1", APIKey: "test-key"},
		thread, "where is the order status?", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "turn call step turn text" {
		t.Fatalf("events = %s", got)
	}
	if step := events[2]; step.Tool != "search_types" || step.Input != "OrdSt" || len(step.Hits) != 1 || step.Hits[0].Title != "OrderStatus" {
		t.Errorf("step = %+v", step)
	}
	if text := events[4]; !strings.Contains(text.Text, "order.go:3") {
		t.Errorf("text = %+v", text)
	}
}

func TestAskOpenAIStreaming(t *testing.T) {
	setupIndex(t)
	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []map[string]any `json:"messages"`
			Stream   bool             `json:"stream"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if !req.Stream {
			t.Error("request is not streamed")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		turn++
		var chunks []string
		switch turn {
		case 1:
			chunks = []string{
				`{"choices":[{"delta":{"reasoning_content":"Look for "}}]}`,
				`{"choices":[{"delta":{"reasoning_content":"the type."}}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"c1","function":{"name":"search_types","arguments":"{\"que"}}]}}]}`,
				`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ry\":\"OrdSt\"}"}}]}}]}`,
			}
		default:
			prev := req.Messages[len(req.Messages)-2]
			calls, _ := prev["tool_calls"].([]any)
			if prev["reasoning_content"] != "Look for the type." || len(calls) != 1 ||
				calls[0].(map[string]any)["function"].(map[string]any)["arguments"] != `{"query":"OrdSt"}` {
				t.Errorf("assistant message not reassembled: %v", prev)
			}
			chunks = []string{`{"choices":[{"delta":{"content":"Found "}}]}`, `{"choices":[{"delta":{"content":"it."}}]}`}
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "data: %s\n\n", c)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var events []Event
	err := Ask(context.Background(), Config{Provider: "openai", Model: "m", BaseURL: server.URL, APIKey: "k"}, nil,
		"q", func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(events); got != "turn thinking thinking call step turn text text" {
		t.Errorf("events = %s", got)
	}
}

func kinds(events []Event) string {
	var k []string
	for _, e := range events {
		k = append(k, e.Kind)
	}
	return strings.Join(k, " ")
}

func TestRunToolValidates(t *testing.T) {
	setupIndex(t)
	if _, _, _, err := runTool(context.Background(), "read_file", []byte(`{"file":"x"}`)); err == nil {
		t.Error("missing parameters should fail")
	}
	if _, _, _, err := runTool(context.Background(), "search_types", []byte(`{"query":`)); err == nil {
		t.Error("truncated JSON should fail")
	}
	if _, _, _, err := runTool(context.Background(), "rm_rf", []byte(`{}`)); err == nil {
		t.Error("unknown tool should fail")
	}
	root := code.Roots()[0].ID
	out, hits, _, err := runTool(context.Background(), "read_file", []byte(`{"file":"`+root+`/order.go","start_line":3,"end_line":3}`))
	if err != nil || out != "3\ttype OrderStatus int\n(2 more lines)\n" || len(hits) != 1 {
		t.Errorf("read_file = %q, %v, %v", out, hits, err)
	}
	if _, _, _, err := runTool(context.Background(), "read_file", []byte(`{"file":"`+root+`/../../etc/passwd","start_line":1,"end_line":1}`)); err == nil {
		t.Error("read_file outside the root should fail")
	}
}

func TestAskWithoutKey(t *testing.T) {
	if err := Ask(context.Background(), Config{Provider: "openai"}, nil, "q", func(Event) {}); err != ErrNoKey {
		t.Errorf("err = %v", err)
	}
}
