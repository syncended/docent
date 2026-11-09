package provider

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Choice is a model offered by the picker: what the provider lists, or
// what Docent knows, described as far as either says.
type Choice struct {
	Model
	// Known reports a model Docent knows by its id.
	Known bool `json:"known"`
	// Tunable reports a model whose reasoning effort can be chosen.
	Tunable bool `json:"tunable"`
}

// Describe turns a model served by an endpoint into a picker choice,
// filling in what the catalog knows where the listing says nothing.
func Describe(e Endpoint, listed Model) Choice {
	m, known := Lookup(e, listed.ID)
	if listed.Name != "" {
		m.Name = listed.Name
	}
	if listed.Context > 0 {
		m.Context = listed.Context
	}
	if listed.Output > 0 {
		m.Output = listed.Output
	}
	m.Reasoning = m.Reasoning || listed.Reasoning
	if len(listed.Efforts) > 0 {
		m.Efforts = listed.Efforts
	}
	return Choice{Model: m, Known: known, Tunable: Tunable(m)}
}

// Catalog returns the models Docent knows for an endpoint as choices.
func Catalog(e Endpoint) []Choice {
	var out []Choice
	for _, m := range Models(e) {
		out = append(out, Choice{Model: m, Known: true, Tunable: Tunable(m)})
	}
	return out
}

// Order sorts choices for the picker: the recommended model first, then
// the known ones, newest first, then the rest by id.
func Order(p Provider, choices []Choice) []Choice {
	out := slices.Clone(choices)
	rank := func(c Choice) int {
		switch {
		case c.ID == p.Recommended:
			return 0
		case c.Known:
			return 1
		}
		return 2
	}
	slices.SortStableFunc(out, func(a, b Choice) int {
		if r := cmp.Compare(rank(a), rank(b)); r != 0 {
			return r
		}
		if a.Known {
			if r := strings.Compare(b.Released, a.Released); r != 0 {
				return r
			}
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// List asks the provider which models are available at baseURL with key
// ("" for none) and describes them. Models that cannot call tools are left
// out where the listing says so.
func List(ctx context.Context, p Provider, e Endpoint, baseURL, key string) ([]Choice, error) {
	base := strings.TrimRight(baseURL, "/")
	var listed []Model
	var err error
	switch p.Listing {
	case ListAnthropic:
		listed, err = listAnthropic(ctx, base, key)
	case ListOllama:
		listed, err = listOllama(ctx, serverRoot(base))
	case ListLMStudio:
		listed, err = listLMStudio(ctx, serverRoot(base), key)
		if err != nil {
			listed, err = listOpenAI(ctx, base, key)
		}
	default:
		listed, err = listOpenAI(ctx, base, key)
	}
	if err != nil {
		return nil, err
	}
	out := make([]Choice, len(listed))
	for i, m := range listed {
		out[i] = Describe(e, m)
	}
	return out, nil
}

// serverRoot is a local server's address without the OpenAI-compatible
// "/v1" suffix, where its own API lives.
func serverRoot(base string) string {
	return strings.TrimSuffix(base, "/v1")
}

func getJSON(ctx context.Context, url string, header http.Header, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header = header
	return doJSON(req, out)
}

func doJSON(req *http.Request, out any) error {
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", res.Status, errorMessage(data))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("unexpected model list: %.200s", data)
	}
	return nil
}

func errorMessage(data []byte) string {
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
	return strings.TrimSpace(fmt.Sprintf("%.200s", data))
}

// listOpenAI reads GET {base}/models. Beyond the id, providers add their
// own fields: names, context windows, reasoning and tool support.
func listOpenAI(ctx context.Context, base, key string) ([]Model, error) {
	header := http.Header{}
	if key != "" {
		header.Set("Authorization", "Bearer "+key)
	}
	var out struct {
		Data []struct {
			ID                string `json:"id"`
			Name              string `json:"name"`
			DisplayName       string `json:"display_name"`
			ContextLength     int    `json:"context_length"`
			ContextWindow     int    `json:"context_window"`
			MaxOutputTokens   int    `json:"max_output_tokens"`
			SupportsReasoning bool   `json:"supports_reasoning"`
			// OpenRouter
			SupportedParameters []string `json:"supported_parameters"`
			TopProvider         struct {
				MaxCompletionTokens int `json:"max_completion_tokens"`
			} `json:"top_provider"`
			Reasoning struct {
				SupportedEfforts []string `json:"supported_efforts"`
			} `json:"reasoning"`
		} `json:"data"`
	}
	if err := getJSON(ctx, base+"/models", header, &out); err != nil {
		return nil, err
	}
	var models []Model
	for _, d := range out.Data {
		if d.SupportedParameters != nil && !slices.Contains(d.SupportedParameters, "tools") {
			continue
		}
		models = append(models, Model{
			// Gemini lists "models/gemini-…" but takes the bare id.
			ID:        strings.TrimPrefix(d.ID, "models/"),
			Name:      cmp.Or(d.DisplayName, d.Name),
			Context:   max(d.ContextLength, d.ContextWindow),
			Output:    max(d.MaxOutputTokens, d.TopProvider.MaxCompletionTokens),
			Reasoning: d.SupportsReasoning || slices.Contains(d.SupportedParameters, "reasoning"),
			Efforts:   d.Reasoning.SupportedEfforts,
		})
	}
	return models, nil
}

// listAnthropic reads the Anthropic Models API.
func listAnthropic(ctx context.Context, base, key string) ([]Model, error) {
	header := http.Header{}
	header.Set("x-api-key", key)
	header.Set("anthropic-version", "2023-06-01")
	var out struct {
		Data []struct {
			ID             string `json:"id"`
			DisplayName    string `json:"display_name"`
			MaxInputTokens int    `json:"max_input_tokens"`
			MaxTokens      int    `json:"max_tokens"`
		} `json:"data"`
	}
	if err := getJSON(ctx, base+"/v1/models?limit=1000", header, &out); err != nil {
		return nil, err
	}
	var models []Model
	for _, d := range out.Data {
		models = append(models, Model{ID: d.ID, Name: d.DisplayName, Context: d.MaxInputTokens, Output: d.MaxTokens})
	}
	return models, nil
}

// listOllama lists the local models and asks each for its context length
// and capabilities, leaving out those that cannot call tools.
func listOllama(ctx context.Context, root string) ([]Model, error) {
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := getJSON(ctx, root+"/api/tags", http.Header{}, &tags); err != nil {
		return nil, err
	}
	models := make([]Model, len(tags.Models))
	skip := make([]bool, len(tags.Models))
	var g errgroup.Group
	g.SetLimit(4)
	for i, t := range tags.Models {
		g.Go(func() error {
			models[i] = Model{ID: t.Name}
			body, _ := json.Marshal(map[string]string{"model": t.Name})
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/api/show", strings.NewReader(string(body)))
			if err != nil {
				return nil
			}
			req.Header.Set("Content-Type", "application/json")
			var show struct {
				Capabilities []string       `json:"capabilities"`
				ModelInfo    map[string]any `json:"model_info"`
			}
			if doJSON(req, &show) != nil {
				return nil
			}
			if show.Capabilities != nil && !slices.Contains(show.Capabilities, "tools") {
				skip[i] = true
			}
			models[i].Reasoning = slices.Contains(show.Capabilities, "thinking")
			for k, v := range show.ModelInfo {
				if n, ok := v.(float64); ok && strings.HasSuffix(k, ".context_length") {
					models[i].Context = int(n)
				}
			}
			return nil
		})
	}
	g.Wait()
	var out []Model
	for i, m := range models {
		if !skip[i] {
			out = append(out, m)
		}
	}
	return out, nil
}

// listLMStudio reads LM Studio's own model list, which tells tool use
// and context length apart from embedding models.
func listLMStudio(ctx context.Context, root, key string) ([]Model, error) {
	header := http.Header{}
	if key != "" {
		header.Set("Authorization", "Bearer "+key)
	}
	var out struct {
		Models []struct {
			Key              string `json:"key"`
			Type             string `json:"type"`
			DisplayName      string `json:"display_name"`
			MaxContextLength int    `json:"max_context_length"`
			Capabilities     struct {
				TrainedForToolUse *bool           `json:"trained_for_tool_use"`
				Reasoning         json.RawMessage `json:"reasoning"`
			} `json:"capabilities"`
		} `json:"models"`
	}
	if err := getJSON(ctx, root+"/api/v1/models", header, &out); err != nil {
		return nil, err
	}
	if out.Models == nil {
		return nil, fmt.Errorf("unexpected LM Studio model list")
	}
	var models []Model
	for _, m := range out.Models {
		if m.Type != "" && m.Type != "llm" {
			continue
		}
		if t := m.Capabilities.TrainedForToolUse; t != nil && !*t {
			continue
		}
		r := strings.TrimSpace(string(m.Capabilities.Reasoning))
		models = append(models, Model{
			ID: m.Key, Name: m.DisplayName, Context: m.MaxContextLength,
			Reasoning: r != "" && r != "null" && r != "false",
		})
	}
	return models, nil
}

var (
	listMu    sync.Mutex
	listCache = map[string][]Choice{}
)

// ListCached is List, remembered until restart per provider, endpoint
// address and key. A server on this machine is asked every time: models come and go
// there as they are pulled and removed.
func ListCached(ctx context.Context, p Provider, e Endpoint, baseURL, key string) ([]Choice, error) {
	if host, _, _ := hostPath(baseURL); host == "localhost" || strings.HasPrefix(host, "localhost:") {
		return List(ctx, p, e, baseURL, key)
	}
	k := p.ID + "\x00" + baseURL + "\x00" + key
	listMu.Lock()
	cached, ok := listCache[k]
	listMu.Unlock()
	if ok {
		return cached, nil
	}
	out, err := List(ctx, p, e, baseURL, key)
	if err != nil {
		return nil, err
	}
	listMu.Lock()
	listCache[k] = out
	listMu.Unlock()
	return out, nil
}
