package agent

import (
	"context"
	"fmt"
	"slices"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

const defaultAnthropicModel = "claude-opus-5-5"

// fallbackModels accept server-side refusal fallbacks ("fallbacks": "default").
var fallbackModels = []string{"claude-fable-5-1", "claude-opus-5-5", "claude-opus-5", "claude-sonnet-5-5"}

// askAnthropic runs the agent over the Anthropic Messages API, streaming
// answer text as it is generated.
func askAnthropic(ctx context.Context, cfg Config, question string, emit func(Event)) error {
	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := anthropic.NewClient(opts...)

	model := cfg.Model
	if model == "" {
		model = defaultAnthropicModel
	}
	params := anthropic.BetaMessageNewParams{
		Model:     model,
		MaxTokens: 64000,
		System:    []anthropic.BetaTextBlockParam{{Text: systemPrompt}},
		Tools:     anthropicTools(),
		OutputConfig: anthropic.BetaOutputConfigParam{
			Effort: anthropic.BetaOutputConfigEffortMedium,
		},
		// Summarized reasoning is shown live, so a search never looks stuck.
		Thinking: anthropic.BetaThinkingConfigParamUnion{OfAdaptive: &anthropic.BetaThinkingConfigAdaptiveParam{
			Display: anthropic.BetaThinkingConfigAdaptiveDisplaySummarized,
		}},
		// Caches the system prompt, tools and the conversation so far.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		Messages: []anthropic.BetaMessageParam{
			anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(question)),
		},
	}
	// A refused request is re-served by a fallback model in the same call.
	// Only on the first-party API: proxies may not accept the field.
	if cfg.BaseURL == "" && slices.Contains(fallbackModels, model) {
		params.Betas = []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01}
		params.Fallbacks = anthropic.BetaFallbacksParamUnion{OfDefault: constant.ValueOf[constant.Default]()}
	}

	for step := 0; ; step++ {
		if step == maxSteps {
			params.Messages = append(params.Messages, anthropic.NewBetaUserMessage(
				anthropic.NewBetaTextBlock("Stop searching now and answer with what you have found."),
			))
			params.Tools = nil
		}
		msg, err := streamMessage(ctx, client, params, emit)
		if err != nil {
			return err
		}
		params.Messages = append(params.Messages, msg.ToParam())

		switch msg.StopReason {
		case anthropic.BetaStopReasonRefusal:
			return fmt.Errorf("the model declined to answer")
		case anthropic.BetaStopReasonMaxTokens:
			return fmt.Errorf("the answer was cut off at the token limit")
		case anthropic.BetaStopReasonToolUse:
		default:
			return nil
		}

		var results []anthropic.BetaContentBlockParamUnion
		for _, block := range msg.Content {
			use, ok := block.AsAny().(anthropic.BetaToolUseBlock)
			if !ok {
				continue
			}
			result, isError := callTool(ctx, use.Name, []byte(use.JSON.Input.Raw()), emit)
			results = append(results, anthropic.NewBetaToolResultBlock(use.ID, result, isError))
		}
		params.Messages = append(params.Messages, anthropic.NewBetaUserMessage(results...))
	}
}

func streamMessage(ctx context.Context, client anthropic.Client, params anthropic.BetaMessageNewParams, emit func(Event)) (anthropic.BetaMessage, error) {
	emit(Event{Kind: "turn"})
	stream := client.Beta.Messages.NewStreaming(ctx, params)
	msg := anthropic.BetaMessage{}
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return msg, err
		}
		if delta, ok := ev.AsAny().(anthropic.BetaRawContentBlockDeltaEvent); ok {
			switch d := delta.Delta.AsAny().(type) {
			case anthropic.BetaTextDelta:
				emit(Event{Kind: "text", Text: d.Text})
			case anthropic.BetaThinkingDelta:
				emit(Event{Kind: "thinking", Text: d.Thinking})
			}
		}
	}
	return msg, stream.Err()
}

func anthropicTools() []anthropic.BetaToolUnionParam {
	out := make([]anthropic.BetaToolUnionParam, len(tools))
	for i, t := range tools {
		out[i] = anthropic.BetaToolUnionParam{OfTool: &anthropic.BetaToolParam{
			Name:        t.name,
			Description: param.NewOpt(t.description),
			InputSchema: anthropic.BetaToolInputSchemaParam{Properties: t.properties, Required: t.required},
			// Inputs arrive as they are generated; runTool validates them.
			EagerInputStreaming: param.NewOpt(true),
		}}
	}
	return out
}
