// Package provider is Docent's catalog of model providers and their models:
// the presets the built-in agent talks to, the models it knows, and the
// recognition of a known model by its id on any endpoint (DESIGN.md,
// Providers and models).
package provider

//go:generate go run ./gen

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
)

// Kind groups providers in Settings.
type Kind string

const (
	Vendor Kind = "vendor" // a vendor serving its own models
	Host   Kind = "host"   // a host or aggregator serving many vendors' models
	Local  Kind = "local"  // a server on this machine
	Custom Kind = "custom" // any other OpenAI-compatible endpoint
)

// API is the protocol a provider is spoken to over.
type API string

const (
	Anthropic API = "anthropic" // Anthropic Messages API
	Responses API = "responses" // OpenAI Responses API
	Chat      API = "chat"      // OpenAI-compatible Chat Completions
)

// Listing is how a provider's live model list is fetched.
type Listing string

const (
	ListOpenAI    Listing = "openai"    // GET {base}/models
	ListAnthropic Listing = "anthropic" // GET {base}/v1/models
	ListOllama    Listing = "ollama"    // /api/tags and /api/show on the server
	ListLMStudio  Listing = "lmstudio"  // /api/v1/models on the server
)

// Key says whether an endpoint needs an API key.
type Key string

const (
	KeyRequired Key = "required"
	KeyOptional Key = "optional"
	KeyNone     Key = "none"
)

// Provider is a preset: where a provider is, how to talk to it and which
// models it serves.
type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind Kind   `json:"kind"`
	API  API    `json:"api"`
	// Endpoints are the regions and plans; the first is the default.
	Endpoints []Endpoint `json:"endpoints"`
	// Recommended is the model chosen by default for code search.
	Recommended string  `json:"recommended,omitempty"`
	Listing     Listing `json:"-"`
	// EffortParam is where a Chat Completions request carries the reasoning
	// effort: "reasoning_effort", or "reasoning.effort" for a nested field.
	EffortParam string `json:"-"`
	// Body holds fields every Chat Completions request to the provider carries.
	Body map[string]any `json:"-"`
}

// Endpoint is one region or plan of a provider.
type Endpoint struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	URL   string `json:"url"`
	Key   Key    `json:"key"`
	// Catalogs are the models.dev providers whose models it serves.
	Catalogs []string `json:"-"`
}

// Model is a model as Docent knows it.
type Model struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Context   int    `json:"context,omitempty"`
	Output    int    `json:"output,omitempty"`
	Reasoning bool   `json:"reasoning,omitempty"`
	// Efforts are the reasoning effort values the model accepts, in order,
	// as the provider spells them.
	Efforts  []string `json:"efforts,omitempty"`
	Released string   `json:"released,omitempty"`
}

// Levels are Docent's reasoning effort levels, the same for every model.
var Levels = []string{"low", "medium", "high"}

// DefaultLevel is the effort Docent uses for code search unless the user
// picks another: deep enough to follow code, without the maximum some
// providers default to.
const DefaultLevel = "medium"

const reasoningEffort = "reasoning_effort"

var providers = []Provider{
	{
		ID: "openai", Name: "OpenAI", Kind: Vendor, API: Responses, Recommended: "gpt-6.1-sol", Listing: ListOpenAI,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.openai.com/v1", Key: KeyRequired, Catalogs: []string{"openai"}}},
	},
	{
		ID: "anthropic", Name: "Anthropic", Kind: Vendor, API: Anthropic, Recommended: "claude-opus-5-5", Listing: ListAnthropic,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.anthropic.com", Key: KeyRequired, Catalogs: []string{"anthropic"}}},
	},
	{
		ID: "google", Name: "Google", Kind: Vendor, API: Chat, Recommended: "gemini-4-argon", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "Gemini API", URL: "https://generativelanguage.googleapis.com/v1beta/openai", Key: KeyRequired, Catalogs: []string{"google"}}},
	},
	{
		ID: "xai", Name: "xAI", Kind: Vendor, API: Chat, Recommended: "grok-4.7", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.x.ai/v1", Key: KeyRequired, Catalogs: []string{"xai"}}},
	},
	{
		ID: "mistral", Name: "Mistral", Kind: Vendor, API: Chat, Recommended: "mistral-large-4", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.mistral.ai/v1", Key: KeyRequired, Catalogs: []string{"mistral"}}},
	},
	{
		ID: "deepseek", Name: "DeepSeek", Kind: Vendor, API: Chat, Recommended: "deepseek-v4-pro", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.deepseek.com", Key: KeyRequired, Catalogs: []string{"deepseek"}}},
	},
	{
		ID: "zai", Name: "Z.ai", Kind: Vendor, API: Chat, Recommended: "glm-5.3", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{
			{ID: "intl", Label: "International", URL: "https://api.z.ai/api/paas/v4", Key: KeyRequired, Catalogs: []string{"zai"}},
			{ID: "cn", Label: "China", URL: "https://open.bigmodel.cn/api/paas/v4", Key: KeyRequired, Catalogs: []string{"zhipuai"}},
			{ID: "coding", Label: "Coding Plan", URL: "https://api.z.ai/api/coding/paas/v4", Key: KeyRequired, Catalogs: []string{"zai-coding-plan"}},
			{ID: "coding-cn", Label: "Coding Plan (China)", URL: "https://open.bigmodel.cn/api/coding/paas/v4", Key: KeyRequired, Catalogs: []string{"zhipuai-coding-plan"}},
		},
	},
	{
		ID: "moonshot", Name: "Moonshot (Kimi)", Kind: Vendor, API: Chat, Recommended: "kimi-k3.1-pro", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{
			{ID: "intl", Label: "International", URL: "https://api.moonshot.ai/v1", Key: KeyRequired, Catalogs: []string{"moonshotai"}},
			{ID: "cn", Label: "China", URL: "https://api.moonshot.cn/v1", Key: KeyRequired, Catalogs: []string{"moonshotai-cn"}},
		},
	},
	{
		ID: "alibaba", Name: "Alibaba (Qwen)", Kind: Vendor, API: Chat, Recommended: "qwen3.8-max", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{
			{ID: "intl", Label: "International", URL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1", Key: KeyRequired, Catalogs: []string{"alibaba"}},
			{ID: "cn", Label: "China", URL: "https://dashscope.aliyuncs.com/compatible-mode/v1", Key: KeyRequired, Catalogs: []string{"alibaba-cn"}},
			{ID: "coding", Label: "Coding Plan", URL: "https://coding-intl.dashscope.aliyuncs.com/v1", Key: KeyRequired, Catalogs: []string{"alibaba-coding-plan"}},
		},
	},
	{
		ID: "minimax", Name: "MiniMax", Kind: Vendor, API: Chat, Recommended: "MiniMax-M3", Listing: ListOpenAI, EffortParam: reasoningEffort,
		// Reasoning comes in reasoning_content rather than <think> tags.
		Body: map[string]any{"reasoning_split": true},
		Endpoints: []Endpoint{
			{ID: "intl", Label: "International", URL: "https://api.minimax.io/v1", Key: KeyRequired, Catalogs: []string{"minimax", "minimax-coding-plan"}},
			{ID: "cn", Label: "China", URL: "https://api.minimaxi.com/v1", Key: KeyRequired, Catalogs: []string{"minimax-cn"}},
		},
	},
	{
		ID: "openrouter", Name: "OpenRouter", Kind: Host, API: Chat, Recommended: "anthropic/claude-opus-5.5", Listing: ListOpenAI, EffortParam: "reasoning.effort",
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://openrouter.ai/api/v1", Key: KeyRequired, Catalogs: []string{"openrouter"}}},
	},
	{
		ID: "groq", Name: "Groq", Kind: Host, API: Chat, Recommended: "openai/gpt-oss-120b", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.groq.com/openai/v1", Key: KeyRequired, Catalogs: []string{"groq"}}},
	},
	{
		ID: "together", Name: "Together", Kind: Host, API: Chat, Recommended: "moonshotai/Kimi-K3.1-Pro", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.together.ai/v1", Key: KeyRequired, Catalogs: []string{"togetherai"}}},
	},
	{
		ID: "fireworks", Name: "Fireworks", Kind: Host, API: Chat, Recommended: "accounts/fireworks/models/glm-5p3", Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "api", Label: "API", URL: "https://api.fireworks.ai/inference/v1", Key: KeyRequired, Catalogs: []string{"fireworks-ai"}}},
	},
	{
		ID: "ollama", Name: "Ollama", Kind: Local, API: Chat, Listing: ListOllama, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{
			{ID: "local", Label: "This machine", URL: "http://localhost:11434/v1", Key: KeyNone},
			{ID: "cloud", Label: "Ollama Cloud", URL: "https://ollama.com/v1", Key: KeyRequired, Catalogs: []string{"ollama-cloud"}},
		},
	},
	{
		ID: "lmstudio", Name: "LM Studio", Kind: Local, API: Chat, Listing: ListLMStudio, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "local", Label: "This machine", URL: "http://localhost:1234/v1", Key: KeyOptional, Catalogs: []string{"lmstudio"}}},
	},
	{
		ID: "custom", Name: "Custom", Kind: Custom, API: Chat, Listing: ListOpenAI, EffortParam: reasoningEffort,
		Endpoints: []Endpoint{{ID: "custom", Label: "Custom", Key: KeyOptional}},
	},
}

// All returns the presets in the order Settings lists them.
func All() []Provider { return providers }

// Find returns the provider with the given id.
func Find(id string) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// EndpointOf returns the provider's endpoint with the given id, or its
// default one.
func EndpointOf(p Provider, id string) Endpoint {
	for _, e := range p.Endpoints {
		if e.ID == id {
			return e
		}
	}
	return p.Endpoints[0]
}

// ForURL returns the preset and endpoint a base URL belongs to, if any:
// the endpoint on the same host whose path is the longest prefix of the
// URL's path, or else the host's first endpoint.
func ForURL(raw string) (Provider, Endpoint, bool) {
	host, path, ok := hostPath(raw)
	if !ok {
		return Provider{}, Endpoint{}, false
	}
	var best Provider
	var bestEndpoint Endpoint
	bestLen := -1
	for _, p := range providers {
		for _, e := range p.Endpoints {
			h, ep, ok := hostPath(e.URL)
			if !ok || h != host {
				continue
			}
			n := 0
			if path == ep || strings.HasPrefix(path, ep+"/") {
				n = len(ep) + 1
			}
			if n > bestLen {
				best, bestEndpoint, bestLen = p, e, n
			}
		}
	}
	return best, bestEndpoint, bestLen >= 0
}

// hostPath splits a URL into its host (with port, loopback addresses as
// "localhost") and its path without a trailing slash.
func hostPath(raw string) (host, path string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	h := strings.ToLower(u.Hostname())
	if h == "127.0.0.1" || h == "::1" {
		h = "localhost"
	}
	if p := u.Port(); p != "" {
		h += ":" + p
	}
	return h, strings.TrimRight(u.Path, "/"), true
}

//go:embed catalog.json
var catalogJSON []byte

var (
	catalogOnce sync.Once
	// catalog holds the bundled models by models.dev provider.
	catalog map[string][]Model
	// known indexes every bundled model by its normalized id, vendors'
	// own entries first.
	known map[string]Model
)

func loadCatalog() {
	catalogOnce.Do(func() {
		if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
			panic("provider: bad catalog.json: " + err.Error())
		}
		known = map[string]Model{}
		add := func(kinds ...Kind) {
			for _, p := range providers {
				if !slices.Contains(kinds, p.Kind) {
					continue
				}
				for _, e := range p.Endpoints {
					for _, c := range e.Catalogs {
						for _, m := range catalog[c] {
							if _, ok := known[normalize(m.ID)]; !ok {
								known[normalize(m.ID)] = m
							}
						}
					}
				}
			}
		}
		add(Vendor)
		add(Host, Local)
	})
}

// Models returns the models Docent knows for an endpoint, newest first.
func Models(e Endpoint) []Model {
	loadCatalog()
	var out []Model
	seen := map[string]bool{}
	for _, c := range e.Catalogs {
		for _, m := range catalog[c] {
			if !seen[m.ID] {
				seen[m.ID] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// Lookup describes a model served by an endpoint: the endpoint's own
// catalog entry if it has one, otherwise the model recognized by its id.
// ok reports whether the model is known; an unknown model has only its id.
func Lookup(e Endpoint, id string) (m Model, ok bool) {
	for _, m := range Models(e) {
		if m.ID == id {
			return m, true
		}
	}
	return Recognize(id)
}

// Recognize describes a model by its id wherever it is served: an
// aggregator's namespace, a tag and letter case are ignored, and a newer
// version of a known model takes after it (kimi-k3.2 after kimi-k3).
func Recognize(id string) (Model, bool) {
	loadCatalog()
	n := normalize(id)
	if m, ok := known[n]; ok {
		m.ID = id
		return m, true
	}
	var best Model
	bestLen := 0
	for k, m := range known {
		if len(k) > bestLen && strings.HasPrefix(n, k) && strings.ContainsRune("-._", rune(n[len(k)])) {
			best, bestLen = m, len(k)
		}
	}
	if bestLen == 0 {
		return Model{ID: id}, false
	}
	// Only how it reasons carries over; the name and limits are another model's.
	return Model{ID: id, Reasoning: best.Reasoning, Efforts: best.Efforts}, true
}

var fireworksVersion = regexp.MustCompile(`(\d)p(\d)`)

// normalize reduces a model id to the vendor's spelling:
// "accounts/fireworks/models/kimi-k3p1-pro" and "moonshotai/Kimi-K3.1-Pro"
// both become "kimi-k3.1-pro", "deepseek-v4-pro:0813" "deepseek-v4-pro".
func normalize(id string) string {
	id = strings.ToLower(id)
	id = id[strings.LastIndex(id, "/")+1:]
	if i := strings.IndexByte(id, ':'); i >= 0 {
		id = id[:i]
	}
	return fireworksVersion.ReplaceAllString(id, "$1.$2")
}

// Effort returns the model's own effort value for one of Docent's levels,
// or "" if the model's effort cannot be set. The levels spread over the
// values the model accepts, past "none" and "minimal": with low, high and
// max, medium is high and high is max.
func Effort(m Model, level string) string {
	values := effortValues(m)
	if len(values) == 0 {
		return ""
	}
	if slices.Contains(values, "low") && slices.Contains(values, "medium") && slices.Contains(values, "high") {
		if slices.Contains(Levels, level) {
			return level
		}
		return "medium"
	}
	switch level {
	case "low":
		return values[0]
	case "high":
		return values[len(values)-1]
	default:
		return values[(len(values)-1)/2]
	}
}

// Tunable reports whether the user has an effort to choose for the model.
func Tunable(m Model) bool { return len(effortValues(m)) > 1 }

// effortScale orders the effort values providers use; listings give them
// in either direction (OpenRouter lists "max" first).
var effortScale = []string{"low", "medium", "high", "xhigh", "max"}

// effortValues returns the values the levels spread over, lowest first:
// those on the scale, without "none", "minimal" and "default".
func effortValues(m Model) []string {
	var out []string
	for _, v := range m.Efforts {
		if slices.Contains(effortScale, v) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	slices.SortFunc(out, func(a, b string) int {
		return slices.Index(effortScale, a) - slices.Index(effortScale, b)
	})
	return out
}
