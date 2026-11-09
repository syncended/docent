package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestRecommendedModelsAreKnown(t *testing.T) {
	for _, p := range All() {
		if p.Recommended == "" {
			continue
		}
		if _, ok := Lookup(p.Endpoints[0], p.Recommended); !ok {
			t.Errorf("%s recommends %s, which the catalog does not know", p.ID, p.Recommended)
		}
	}
}

func TestRecognize(t *testing.T) {
	for _, tc := range []struct {
		id      string
		known   bool
		efforts []string
	}{
		{"glm-5.3", true, []string{"low", "high", "max"}},
		{"z-ai/glm-5.3", true, []string{"low", "high", "max"}},
		{"zai-org/GLM-5.3", true, []string{"low", "high", "max"}},
		{"accounts/fireworks/models/kimi-k3p1-pro", true, []string{"low", "high", "max"}},
		{"deepseek-v4-pro:0813", true, []string{"low", "high", "max"}},
		// A newer version takes after the known one.
		{"kimi-k3.2", true, []string{"low", "high", "max"}},
		{"my-finetune", false, nil},
		// A prefix that is not at a boundary is another model.
		{"glm-5x", false, nil},
	} {
		m, ok := Recognize(tc.id)
		if ok != tc.known || m.ID != tc.id {
			t.Errorf("Recognize(%q) = %q, %v; want known %v", tc.id, m.ID, ok, tc.known)
			continue
		}
		if tc.known && !slices.Equal(m.Efforts, tc.efforts) {
			t.Errorf("Recognize(%q) efforts = %v, want %v", tc.id, m.Efforts, tc.efforts)
		}
	}
}

func TestLookupPrefersTheEndpointsCatalog(t *testing.T) {
	// OpenRouter spells DeepSeek's efforts its own way.
	p, _ := Find("openrouter")
	m, ok := Lookup(p.Endpoints[0], "deepseek/deepseek-v4-pro")
	if !ok || !slices.Equal(m.Efforts, []string{"none", "high", "xhigh"}) {
		t.Errorf("efforts on OpenRouter = %v, %v", m.Efforts, ok)
	}
}

func TestEffort(t *testing.T) {
	for _, tc := range []struct {
		efforts []string
		want    [3]string // low, medium, high
		tunable bool
	}{
		{[]string{"low", "medium", "high", "xhigh", "max"}, [3]string{"low", "medium", "high"}, true},
		{[]string{"none", "minimal", "low", "medium", "high"}, [3]string{"low", "medium", "high"}, true},
		{[]string{"low", "high", "max"}, [3]string{"low", "high", "max"}, true},
		{[]string{"low", "medium", "xhigh"}, [3]string{"low", "medium", "xhigh"}, true},
		{[]string{"high", "max"}, [3]string{"high", "high", "max"}, true},
		{[]string{"none", "high"}, [3]string{"high", "high", "high"}, false},
		// OpenRouter lists the highest first.
		{[]string{"max", "high", "low"}, [3]string{"low", "high", "max"}, true},
		{[]string{"high", "low", "none"}, [3]string{"low", "low", "high"}, true},
		{[]string{"none", "default"}, [3]string{"", "", ""}, false},
		{nil, [3]string{"", "", ""}, false},
	} {
		m := Model{Efforts: tc.efforts}
		got := [3]string{Effort(m, "low"), Effort(m, "medium"), Effort(m, "high")}
		if got != tc.want || Tunable(m) != tc.tunable {
			t.Errorf("efforts %v: got %v tunable %v, want %v tunable %v", tc.efforts, got, Tunable(m), tc.want, tc.tunable)
		}
	}
}

func TestForURL(t *testing.T) {
	for _, tc := range []struct{ url, provider, endpoint string }{
		{"https://api.deepseek.com", "deepseek", "api"},
		{"https://api.deepseek.com/v1/", "deepseek", "api"},
		{"api.z.ai/api/coding/paas/v4", "zai", "coding"},
		{"https://api.z.ai/api/paas/v4", "zai", "intl"},
		{"https://open.bigmodel.cn/api/coding/paas/v4", "zai", "coding-cn"},
		{"http://127.0.0.1:11434/v1", "ollama", "local"},
		{"http://localhost:1234/v1", "lmstudio", "local"},
		{"https://openrouter.ai/api/v1", "openrouter", "api"},
		{"https://llm.example.com/v1", "", ""},
		{"", "", ""},
	} {
		p, e, ok := ForURL(tc.url)
		if ok != (tc.provider != "") || p.ID != tc.provider || e.ID != tc.endpoint {
			t.Errorf("ForURL(%q) = %s/%s %v, want %s/%s", tc.url, p.ID, e.ID, ok, tc.provider, tc.endpoint)
		}
	}
}

func TestListOpenAI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("unexpected request %s, auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "glm-5.3"},
			{"id": "kimi-k3", "context_length": 262144, "supports_reasoning": true},
			{"id": "embed-1", "supported_parameters": []string{"max_tokens"}},
			{"id": "brand-new"},
		}})
	}))
	defer server.Close()
	p, _ := Find("custom")
	got, err := List(context.Background(), p, p.Endpoints[0], server.URL+"/v1/", "k")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	if !slices.Equal(ids, []string{"glm-5.3", "kimi-k3", "brand-new"}) {
		t.Fatalf("ids = %v", ids)
	}
	if !got[0].Known || got[0].Name != "GLM-5.3" || !got[0].Tunable {
		t.Errorf("glm-5.3 = %+v, want it described from the catalog", got[0])
	}
	if got[1].Context != 262144 {
		t.Errorf("kimi-k3 context = %d, want the listing's", got[1].Context)
	}
	if got[2].Known || got[2].Tunable {
		t.Errorf("brand-new = %+v, want unknown", got[2])
	}
}

func TestListOllama(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "glm-5.3:latest"}, {"name": "nomic-embed"}}})
		case "/api/show":
			var in struct{ Model string }
			json.NewDecoder(r.Body).Decode(&in)
			if in.Model == "nomic-embed" {
				json.NewEncoder(w).Encode(map[string]any{"capabilities": []string{"embedding"}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"capabilities": []string{"completion", "tools", "thinking"},
				"model_info":   map[string]any{"glm.context_length": 131072},
			})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	p, _ := Find("ollama")
	got, err := List(context.Background(), p, p.Endpoints[0], server.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "glm-5.3:latest" || got[0].Context != 131072 || !got[0].Known || !got[0].Reasoning {
		t.Errorf("got %+v", got)
	}
}
