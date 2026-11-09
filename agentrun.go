package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/syncended/docent/core/agent"
	"github.com/syncended/docent/core/config"
	"github.com/syncended/docent/core/provider"
	"github.com/syncended/docent/core/secrets"
)

// agentEvent is an agent.Event tagged with the run it belongs to, so the UI
// can drop events from a run it has replaced.
type agentEvent struct {
	Run   int64       `json:"run"`
	Event agent.Event `json:"event"`
}

var (
	runSeq    atomic.Int64
	runMu     sync.Mutex
	cancelRun context.CancelFunc
)

// defaultProvider is the provider of a config that names none.
const defaultProvider = "anthropic"

// activeProvider returns the active provider preset and its settings.
func activeProvider(cfg *config.Config) (provider.Provider, config.ProviderSettings) {
	p, ok := provider.Find(cfg.Agent.Provider)
	if !ok {
		p, _ = provider.Find(defaultProvider)
	}
	return p, cfg.Agent.Providers[p.ID]
}

// agentConfig resolves the active provider's settings into what the agent
// needs: the address, the model and how it reasons.
func agentConfig(cfg *config.Config) (agent.Config, error) {
	p, s := activeProvider(cfg)
	e := provider.EndpointOf(p, s.Endpoint)
	base := cmp.Or(strings.TrimSpace(s.BaseURL), e.URL)
	if base == "" {
		return agent.Config{}, errors.New("set the base URL of the custom provider in Settings")
	}
	model := cmp.Or(strings.TrimSpace(s.Model), p.Recommended)
	if model == "" {
		return agent.Config{}, fmt.Errorf("pick a model for %s in Settings", p.Name)
	}
	key, err := secrets.APIKey(p.ID)
	if err != nil {
		return agent.Config{}, err
	}
	if key == "" && e.Key == provider.KeyRequired {
		return agent.Config{}, agent.ErrNoKey
	}
	m, _ := provider.Lookup(e, model)
	return agent.Config{
		API:         string(p.API),
		BaseURL:     base,
		Model:       model,
		APIKey:      key,
		Effort:      provider.Effort(m, cmp.Or(s.Effort, provider.DefaultLevel)),
		EffortParam: p.EffortParam,
		Body:        p.Body,
	}, nil
}

// migrateAgent moves the agent settings from before per-provider settings
// into the new layout, saving the config if it changed. "openai" then meant
// any OpenAI-compatible API: with another provider's address it becomes
// that preset, or Custom, and the stored key moves along.
func migrateAgent(cfg *config.Config) error {
	a := &cfg.Agent
	if a.Model == "" && a.BaseURL == "" {
		return nil
	}
	id := cmp.Or(a.Provider, defaultProvider)
	s := config.ProviderSettings{Model: a.Model, BaseURL: a.BaseURL}
	if id == "openai" && a.BaseURL != "" {
		if p, e, ok := provider.ForURL(a.BaseURL); ok {
			id, s.Endpoint, s.BaseURL = p.ID, e.ID, ""
		} else {
			id = "custom"
		}
		if id != "openai" {
			key, err := secrets.APIKey("openai")
			if err != nil {
				return err
			}
			if key != "" {
				if err := secrets.SetAPIKey(id, key); err != nil {
					return err
				}
				if err := secrets.DeleteAPIKey("openai"); err != nil {
					return err
				}
			}
		}
	}
	if a.Providers == nil {
		a.Providers = map[string]config.ProviderSettings{}
	}
	a.Providers[id] = s
	a.Provider, a.Model, a.BaseURL = id, "", ""
	return config.Save(configPath, cfg)
}

// startAgent cancels the current run, if any, and starts a new one in the
// background; thread is the earlier turns the question follows up on. It
// returns the run ID carried by its events.
func startAgent(thread []agent.Turn, question string) (int64, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return 0, err
	}
	agentCfg, err := agentConfig(cfg)
	if err != nil {
		return 0, err
	}
	run := runSeq.Add(1)
	ctx, cancel := context.WithCancel(context.Background())
	runMu.Lock()
	if cancelRun != nil {
		cancelRun()
	}
	cancelRun = cancel
	runMu.Unlock()

	emit := func(e agent.Event) { app.Event.Emit("agent:event", agentEvent{Run: run, Event: e}) }
	go func() {
		defer cancel()
		err := agent.Ask(ctx, agentCfg, thread, question, emit)
		switch {
		case errors.Is(err, context.Canceled):
		case err != nil:
			emit(agent.Event{Kind: "error", Text: err.Error()})
		default:
			emit(agent.Event{Kind: "done"})
		}
	}()
	return run, nil
}

func stopAgent() {
	runMu.Lock()
	defer runMu.Unlock()
	if cancelRun != nil {
		cancelRun()
		cancelRun = nil
	}
}
