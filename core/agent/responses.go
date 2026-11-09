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
)

// responsesOptions are the optional parts of a Responses request, each
// dropped if the server refuses it: reasoning summaries need a verified
// organization, and models that do not reason take no effort.
type responsesOptions struct {
	effort    string
	summary   bool
	encrypted bool
}

// askResponses runs the agent over the OpenAI Responses API. Nothing is
// stored on the server: every request carries the whole conversation,
// reasoning included as encrypted items, so the model keeps its train of
// thought between tool calls.
func askResponses(ctx context.Context, cfg Config, thread []Turn, question string, emit func(Event)) error {
	base := strings.TrimRight(cfg.BaseURL, "/")
	var input []json.RawMessage
	for _, t := range thread {
		input = append(input,
			mustJSON(map[string]string{"role": "user", "content": t.Question}),
			mustJSON(map[string]string{"role": "assistant", "content": t.Answer}),
		)
	}
	input = append(input, mustJSON(map[string]string{"role": "user", "content": question}))
	opts := responsesOptions{effort: cfg.Effort, summary: true, encrypted: true}
	for step := 0; ; step++ {
		final := step == maxSteps
		if final {
			input = append(input, mustJSON(map[string]string{"role": "user", "content": finalNudge}))
		}
		emit(Event{Kind: "turn"})
		var output []json.RawMessage
		var err error
		for attempt := 0; ; attempt++ {
			output, err = createResponse(ctx, cfg, base, opts, input, final, emit)
			if err == nil || attempt == 3 {
				break
			}
			msg := err.Error()
			switch {
			case opts.summary && strings.Contains(msg, "summar"):
				opts.summary = false
			case opts.encrypted && strings.Contains(msg, "encrypted"):
				opts.encrypted = false
			case opts.effort != "" && strings.Contains(msg, "effort"):
				opts.effort = ""
			default:
				return err
			}
		}
		if err != nil {
			return err
		}
		input = append(input, output...)
		calls := 0
		for _, item := range output {
			var call struct {
				Type      string `json:"type"`
				CallID    string `json:"call_id"`
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}
			if json.Unmarshal(item, &call) != nil || call.Type != "function_call" {
				continue
			}
			calls++
			result, _ := callTool(ctx, call.Name, []byte(call.Arguments), emit)
			input = append(input, mustJSON(map[string]string{
				"type": "function_call_output", "call_id": call.CallID, "output": result,
			}))
		}
		if calls == 0 || final {
			return nil
		}
	}
}

// createResponse requests the next response, streaming its text and
// reasoning summaries through emit, and returns its output items.
func createResponse(ctx context.Context, cfg Config, base string, opts responsesOptions, input []json.RawMessage, final bool, emit func(Event)) ([]json.RawMessage, error) {
	body := map[string]any{
		"model":        cfg.Model,
		"instructions": systemPrompt,
		"input":        input,
		"stream":       true,
		"store":        false,
	}
	reasoning := map[string]any{}
	if opts.effort != "" {
		reasoning["effort"] = opts.effort
	}
	if opts.summary {
		reasoning["summary"] = "auto"
	}
	if len(reasoning) > 0 {
		body["reasoning"] = reasoning
	}
	if opts.encrypted {
		body["include"] = []string{"reasoning.encrypted_content"}
	}
	if !final {
		defs := make([]map[string]any, len(tools))
		for i, t := range tools {
			defs[i] = map[string]any{
				"type":        "function",
				"name":        t.name,
				"description": t.description,
				"parameters":  map[string]any{"type": "object", "properties": t.properties, "required": t.required},
				"strict":      false,
			}
		}
		body["tools"] = defs
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/responses", bytes.NewReader(mustJSON(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("%s: %s", res.Status, apiError(data))
	}
	return streamedResponse(res.Body, emit)
}

// streamedResponse reads the Responses event stream up to the completed
// response and returns its output items as received.
func streamedResponse(body io.Reader, emit func(Event)) ([]json.RawMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		data = strings.TrimSpace(data)
		if !ok || data == "" || data == "[DONE]" {
			continue
		}
		var ev struct {
			Type         string `json:"type"`
			Delta        string `json:"delta"`
			SummaryIndex int    `json:"summary_index"`
			Message      string `json:"message"`
			Response     struct {
				Output []json.RawMessage `json:"output"`
				Error  *struct {
					Message string `json:"message"`
				} `json:"error"`
				IncompleteDetails *struct {
					Reason string `json:"reason"`
				} `json:"incomplete_details"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &ev); err != nil {
			return nil, fmt.Errorf("unexpected stream event: %.200s", data)
		}
		switch ev.Type {
		case "response.output_text.delta":
			emit(Event{Kind: "text", Text: ev.Delta})
		case "response.reasoning_summary_text.delta":
			emit(Event{Kind: "thinking", Text: ev.Delta})
		case "response.reasoning_summary_part.added":
			if ev.SummaryIndex > 0 {
				emit(Event{Kind: "thinking", Text: "\n\n"})
			}
		case "response.completed":
			return ev.Response.Output, nil
		case "response.incomplete":
			if d := ev.Response.IncompleteDetails; d != nil && d.Reason == "max_output_tokens" {
				return nil, fmt.Errorf("the answer was cut off at the token limit")
			}
			if d := ev.Response.IncompleteDetails; d != nil && d.Reason == "content_filter" {
				return nil, fmt.Errorf("the model declined to answer")
			}
			return ev.Response.Output, nil
		case "response.failed":
			if e := ev.Response.Error; e != nil {
				return nil, fmt.Errorf("%s", e.Message)
			}
			return nil, fmt.Errorf("the response failed")
		case "error":
			if ev.Message != "" {
				return nil, fmt.Errorf("%s", ev.Message)
			}
			return nil, fmt.Errorf("%s", apiError([]byte(data)))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("the response stream ended early")
}
