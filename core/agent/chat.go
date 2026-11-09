package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// askChat runs the agent over an OpenAI-compatible Chat Completions API.
func askChat(ctx context.Context, cfg Config, thread []Turn, question string, emit func(Event)) error {
	base := strings.TrimRight(cfg.BaseURL, "/")

	// History is kept as raw JSON so assistant messages go back as received,
	// reasoning and provider-specific fields included.
	history := []json.RawMessage{mustJSON(map[string]string{"role": "system", "content": systemPrompt})}
	for _, t := range thread {
		history = append(history,
			mustJSON(map[string]string{"role": "user", "content": t.Question}),
			mustJSON(map[string]string{"role": "assistant", "content": t.Answer}),
		)
	}
	history = append(history, mustJSON(map[string]string{"role": "user", "content": question}))
	effort := cfg.Effort
	for step := 0; ; step++ {
		final := step == maxSteps
		if final {
			history = append(history, mustJSON(map[string]string{"role": "user", "content": finalNudge}))
		}
		emit(Event{Kind: "turn"})
		var msg chatMessage
		var raw json.RawMessage
		var err error
		// A server may reject what Docent sends by default; each retry drops
		// one thing it named.
		for attempt := 0; ; attempt++ {
			msg, raw, err = chatCompletion(ctx, cfg, base, effort, history, final, emit)
			if err == nil || attempt == 2 {
				break
			}
			if dropped := rejectedReasoning(err); dropped != "" {
				history = without(history, dropped)
				continue
			}
			if effort != "" && strings.Contains(err.Error(), "effort") {
				effort = ""
				continue
			}
			break
		}
		if err != nil {
			return err
		}
		history = append(history, raw)
		if len(msg.ToolCalls) == 0 || final {
			return nil
		}
		for _, call := range msg.ToolCalls {
			result, _ := callTool(ctx, call.Function.Name, []byte(call.Function.Arguments), emit)
			history = append(history, mustJSON(map[string]string{
				"role": "tool", "tool_call_id": call.ID, "content": result,
			}))
		}
	}
}

// rejectedReasoning returns the reasoning field a server refused to take
// back, if the error names one.
func rejectedReasoning(err error) string {
	msg := err.Error()
	for _, f := range []string{"reasoning_content", "reasoning_details"} {
		if strings.Contains(msg, f) {
			return f
		}
	}
	return ""
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
	// ExtraContent carries provider data that must come back with the call,
	// such as Gemini's thought signature.
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type chatMessage struct {
	ToolCalls []toolCall
}

// chatCompletion requests the next assistant message, streaming its text and
// reasoning through emit. It returns the message and its raw JSON for history.
func chatCompletion(ctx context.Context, cfg Config, base, effort string, history []json.RawMessage, final bool, emit func(Event)) (chatMessage, json.RawMessage, error) {
	body := map[string]any{}
	maps.Copy(body, cfg.Body)
	body["model"] = cfg.Model
	body["messages"] = history
	body["stream"] = true
	if !final {
		defs := make([]map[string]any, len(tools))
		for i, t := range tools {
			defs[i] = map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        t.name,
					"description": t.description,
					"parameters":  map[string]any{"type": "object", "properties": t.properties, "required": t.required},
				},
			}
		}
		body["tools"] = defs
	}
	if effort != "" {
		setPath(body, cfg.EffortParam, effort)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(mustJSON(body)))
	if err != nil {
		return chatMessage{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return chatMessage{}, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		return chatMessage{}, nil, fmt.Errorf("%s: %s", res.Status, apiError(data))
	}
	if !strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		// The server ignored "stream": read a whole response.
		return wholeMessage(res.Body, emit)
	}
	return streamedMessage(res.Body, emit)
}

// setPath sets a field of body by a dotted path, creating nested objects.
func setPath(body map[string]any, path string, v any) {
	if path == "" {
		path = "reasoning_effort"
	}
	keys := strings.Split(path, ".")
	m := body
	for _, k := range keys[:len(keys)-1] {
		next, ok := m[k].(map[string]any)
		if !ok {
			next = map[string]any{}
		} else {
			next = maps.Clone(next)
		}
		m[k] = next
		m = next
	}
	m[keys[len(keys)-1]] = v
}

func wholeMessage(body io.Reader, emit func(Event)) (chatMessage, json.RawMessage, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return chatMessage{}, nil, err
	}
	var out struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out.Choices) == 0 {
		return chatMessage{}, nil, fmt.Errorf("unexpected response: %.200s", data)
	}
	raw := out.Choices[0].Message
	var m struct {
		Content          json.RawMessage `json:"content"`
		ReasoningContent string          `json:"reasoning_content"`
		Reasoning        string          `json:"reasoning"`
		ToolCalls        []toolCall      `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return chatMessage{}, nil, err
	}
	var a assembly
	a.reasoning(m.ReasoningContent+m.Reasoning, emit)
	a.content(m.Content, emit)
	return chatMessage{ToolCalls: m.ToolCalls}, raw, nil
}

// assembly accumulates the parts of a streamed assistant message.
type assembly struct {
	text strings.Builder
	// chunks is set when content came as an array of typed chunks (Mistral):
	// thinking and text, sent back the same way.
	chunks   bool
	thinking strings.Builder
	// fields holds each reasoning field received, by name, to go back the
	// same way: DeepSeek, Z.ai, Kimi, MiniMax and Qwen use reasoning_content;
	// Groq, Together and Ollama reasoning.
	fields map[string]*strings.Builder
	// details are OpenRouter's reasoning_details.
	details []map[string]any
	calls   []*toolCall
}

// reasoning reports a piece of reasoning without keeping it.
func (a *assembly) reasoning(s string, emit func(Event)) {
	if s != "" {
		emit(Event{Kind: "thinking", Text: s})
	}
}

func (a *assembly) field(name, s string, emit func(Event)) {
	if s == "" {
		return
	}
	if a.fields == nil {
		a.fields = map[string]*strings.Builder{}
	}
	if a.fields[name] == nil {
		a.fields[name] = &strings.Builder{}
	}
	a.fields[name].WriteString(s)
	a.reasoning(s, emit)
}

// content takes a content delta: a string, or an array of typed chunks.
func (a *assembly) content(raw json.RawMessage, emit func(Event)) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		a.text.WriteString(s)
		if s != "" {
			emit(Event{Kind: "text", Text: s})
		}
		return
	}
	var chunks []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		Thinking []struct {
			Text string `json:"text"`
		} `json:"thinking"`
	}
	if json.Unmarshal(raw, &chunks) != nil {
		return
	}
	a.chunks = true
	for _, c := range chunks {
		switch c.Type {
		case "thinking":
			for _, t := range c.Thinking {
				a.thinking.WriteString(t.Text)
				a.reasoning(t.Text, emit)
			}
		case "text":
			a.text.WriteString(c.Text)
			if c.Text != "" {
				emit(Event{Kind: "text", Text: c.Text})
			}
		}
	}
}

// detail merges a streamed reasoning_details entry (OpenRouter): entries
// with the same index are one, their text pieces joined.
func (a *assembly) detail(d map[string]any, emit func(Event)) {
	i := len(a.details)
	if n, ok := d["index"].(float64); ok {
		i = int(n)
	}
	for len(a.details) <= i {
		a.details = append(a.details, map[string]any{})
	}
	cur := a.details[i]
	for k, v := range d {
		s, isString := v.(string)
		old, hadString := cur[k].(string)
		switch {
		case isString && hadString && (k == "text" || k == "summary" || k == "data"):
			cur[k] = old + s
		default:
			cur[k] = v
		}
		if isString && (k == "text" || k == "summary") {
			a.reasoning(s, emit)
		}
	}
}

func (a *assembly) message() (chatMessage, json.RawMessage) {
	var msg chatMessage
	for _, c := range a.calls {
		msg.ToolCalls = append(msg.ToolCalls, *c)
	}
	raw := map[string]any{"role": "assistant", "content": nil}
	switch {
	case a.chunks:
		var chunks []map[string]any
		if a.thinking.Len() > 0 {
			chunks = append(chunks, map[string]any{"type": "thinking", "thinking": []map[string]any{{"type": "text", "text": a.thinking.String()}}})
		}
		if a.text.Len() > 0 {
			chunks = append(chunks, map[string]any{"type": "text", "text": a.text.String()})
		}
		raw["content"] = chunks
	case a.text.Len() > 0:
		raw["content"] = a.text.String()
	}
	for name, b := range a.fields {
		raw[name] = b.String()
	}
	if len(a.details) > 0 {
		raw["reasoning_details"] = a.details
	}
	if len(msg.ToolCalls) > 0 {
		raw["tool_calls"] = msg.ToolCalls
	}
	return msg, mustJSON(raw)
}

// streamedMessage reads server-sent chunks and assembles the assistant message.
func streamedMessage(body io.Reader, emit func(Event)) (chatMessage, json.RawMessage, error) {
	var a assembly
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		data = strings.TrimSpace(data)
		if !ok || data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          json.RawMessage  `json:"content"`
					ReasoningContent string           `json:"reasoning_content"`
					Reasoning        string           `json:"reasoning"`
					ReasoningDetails []map[string]any `json:"reasoning_details"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
						ExtraContent json.RawMessage `json:"extra_content"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return chatMessage{}, nil, fmt.Errorf("unexpected stream chunk: %.200s", data)
		}
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return chatMessage{}, nil, fmt.Errorf("%s", apiError([]byte(`{"error":`+string(chunk.Error)+`}`)))
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		a.field("reasoning_content", d.ReasoningContent, emit)
		if len(d.ReasoningDetails) > 0 {
			// OpenRouter sends the same reasoning as a plain string too;
			// the details are what goes back.
			for _, det := range d.ReasoningDetails {
				a.detail(det, emit)
			}
		} else {
			a.field("reasoning", d.Reasoning, emit)
		}
		a.content(d.Content, emit)
		for _, tc := range d.ToolCalls {
			for len(a.calls) <= tc.Index {
				a.calls = append(a.calls, &toolCall{Type: "function"})
			}
			c := a.calls[tc.Index]
			if tc.ID != "" {
				c.ID = tc.ID
			}
			if len(tc.ExtraContent) > 0 && string(tc.ExtraContent) != "null" {
				c.ExtraContent = tc.ExtraContent
			}
			c.Function.Name += tc.Function.Name
			c.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return chatMessage{}, nil, err
	}
	msg, raw := a.message()
	return msg, raw, nil
}

// without returns history with a field dropped from every message.
func without(history []json.RawMessage, field string) []json.RawMessage {
	out := make([]json.RawMessage, len(history))
	for i, m := range history {
		var fields map[string]json.RawMessage
		if json.Unmarshal(m, &fields) == nil && fields[field] != nil {
			delete(fields, field)
			m = mustJSON(fields)
		}
		out[i] = m
	}
	return out
}

func apiError(data []byte) string {
	var e struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && len(e.Error) > 0 {
		var nested struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(e.Error, &nested) == nil && nested.Message != "" {
			return nested.Message
		}
		var s string
		if json.Unmarshal(e.Error, &s) == nil && s != "" {
			return s
		}
	}
	return strings.TrimSpace(fmt.Sprintf("%.300s", data))
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
