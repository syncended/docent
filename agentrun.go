package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/syncended/docent/core/agent"
	"github.com/syncended/docent/core/config"
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

// startAgent cancels the current run, if any, and starts a new one in the
// background. It returns the run ID carried by its events.
func startAgent(question string) (int64, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return 0, err
	}
	key, err := secrets.APIKey(cfg.Agent.Provider)
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
		err := agent.Ask(ctx, agent.Config{
			Provider: cfg.Agent.Provider,
			Model:    cfg.Agent.Model,
			BaseURL:  cfg.Agent.BaseURL,
			APIKey:   key,
		}, question, emit)
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
