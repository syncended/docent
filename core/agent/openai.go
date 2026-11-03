package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openAIBaseURL = "https://api.openai.com/v1"

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// askOpenAI runs the agent over an OpenAI-compatible Chat Completions API.
func askOpenAI(ctx context.Context, cfg Config, thread []Turn, question string, emit func(Event)) error {
	if cfg.Model == "" {
		return fmt.Errorf("set a model for the OpenAI-compatible provider in Settings")
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = openAIBaseURL
	}

	// History is kept as raw JSON so assistant messages are sent back exactly
	// as received, including provider-specific fields.
	history := []json.RawMessage{mustJSON(map[string]string{"role": "system", "content": systemPrompt})}
	for _, t := range thread {
		history = append(history,
			mustJSON(map[string]string{"role": "user", "content": t.Question}),
			mustJSON(map[string]string{"role": "assistant", "content": t.Answer}),
		)
	}
	history = append(history, mustJSON(map[string]string{"role": "user", "content": question}))
	for step := 0; ; step++ {
		final := step == maxSteps
		if final {
			history = append(history, mustJSON(map[string]string{
				"role": "user", "content": "Stop searching now and answer with what you have found.",
			}))
		}
		emit(Event{Kind: "turn"})
		msg, raw, err := chatCompletion(ctx, cfg, base, history, final, emit)
		if err != nil && strings.Contains(err.Error(), "reasoning_content") {
			// Some providers return reasoning_content but reject it in input.
			history = withoutReasoning(history)
			msg, raw, err = chatCompletion(ctx, cfg, base, history, final, emit)
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

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatMessage struct {
	Content   string     `json:"content"`
	ToolCalls []toolCall `json:"tool_calls"`
}

// chatCompletion requests the next assistant message, streaming its text and
// reasoning through emit. It returns the message and its raw JSON for history.
func chatCompletion(ctx context.Context, cfg Config, base string, history []json.RawMessage, final bool, emit func(Event)) (chatMessage, json.RawMessage, error) {
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
	toolChoice := "auto"
	if final {
		toolChoice = "none"
	}
	body := mustJSON(map[string]any{
		"model":       cfg.Model,
		"messages":    history,
		"tools":       defs,
		"tool_choice": toolChoice,
		"stream":      true,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return chatMessage{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
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
	var msg chatMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return chatMessage{}, nil, err
	}
	if msg.Content != "" {
		emit(Event{Kind: "text", Text: msg.Content})
	}
	return msg, raw, nil
}

// streamedMessage reads server-sent chunks and assembles the assistant message.
func streamedMessage(body io.Reader, emit func(Event)) (chatMessage, json.RawMessage, error) {
	var content, reasoning strings.Builder
	var calls []*toolCall
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
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					ToolCalls        []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return chatMessage{}, nil, fmt.Errorf("unexpected stream chunk: %.200s", data)
		}
		if chunk.Error != nil {
			return chatMessage{}, nil, fmt.Errorf("%s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		if d.ReasoningContent != "" {
			reasoning.WriteString(d.ReasoningContent)
			emit(Event{Kind: "thinking", Text: d.ReasoningContent})
		}
		if d.Content != "" {
			content.WriteString(d.Content)
			emit(Event{Kind: "text", Text: d.Content})
		}
		for _, tc := range d.ToolCalls {
			for len(calls) <= tc.Index {
				calls = append(calls, &toolCall{Type: "function"})
			}
			c := calls[tc.Index]
			if tc.ID != "" {
				c.ID = tc.ID
			}
			c.Function.Name += tc.Function.Name
			c.Function.Arguments += tc.Function.Arguments
		}
	}
	if err := scanner.Err(); err != nil {
		return chatMessage{}, nil, err
	}

	msg := chatMessage{Content: content.String()}
	for _, c := range calls {
		msg.ToolCalls = append(msg.ToolCalls, *c)
	}
	raw := map[string]any{"role": "assistant", "content": nil}
	if msg.Content != "" {
		raw["content"] = msg.Content
	}
	if reasoning.Len() > 0 {
		raw["reasoning_content"] = reasoning.String()
	}
	if len(msg.ToolCalls) > 0 {
		raw["tool_calls"] = msg.ToolCalls
	}
	return msg, mustJSON(raw), nil
}

func withoutReasoning(history []json.RawMessage) []json.RawMessage {
	out := make([]json.RawMessage, len(history))
	for i, m := range history {
		var fields map[string]json.RawMessage
		if json.Unmarshal(m, &fields) == nil && fields["reasoning_content"] != nil {
			delete(fields, "reasoning_content")
			m = mustJSON(fields)
		}
		out[i] = m
	}
	return out
}

func apiError(data []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
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
