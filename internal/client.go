// Package internal contains the core logic for the bridge.openai plugin.
// It calls the OpenAI Chat Completions API and manages conversation sessions.
package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/orchestra-mcp/plugin-bridge-openai/internal/tools"
)

// CallOpenAI sends a chat completion request to the OpenAI API.
func CallOpenAI(ctx context.Context, opts tools.SpawnOptions) (*tools.ChatResponse, error) {
	apiKey := ""
	baseURL := ""
	if opts.Env != nil {
		apiKey = opts.Env["OPENAI_API_KEY"]
		baseURL = opts.Env["OPENAI_BASE_URL"]
	}

	clientOpts := []option.RequestOption{}
	if apiKey != "" {
		clientOpts = append(clientOpts, option.WithAPIKey(apiKey))
	}
	if baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(clientOpts...)

	model := opts.Model
	if model == "" {
		model = "gpt-4o"
	}

	// Build messages array.
	messages := []openai.ChatCompletionMessageParamUnion{}

	// Add system prompt if provided.
	if opts.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(opts.SystemPrompt))
	}

	// Add conversation history for resumed sessions.
	for _, h := range opts.History {
		switch h.Role {
		case "user":
			messages = append(messages, openai.UserMessage(h.Content))
		case "assistant":
			messages = append(messages, openai.AssistantMessage(h.Content))
		case "system":
			messages = append(messages, openai.SystemMessage(h.Content))
		default:
			messages = append(messages, openai.UserMessage(h.Content))
		}
	}

	// Add the current user prompt.
	messages = append(messages, openai.UserMessage(opts.Prompt))

	start := time.Now()

	completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("openai chat completion: %w", err)
	}

	durationMs := time.Since(start).Milliseconds()

	responseText := ""
	if len(completion.Choices) > 0 {
		responseText = completion.Choices[0].Message.Content
	}

	return &tools.ChatResponse{
		ResponseText: responseText,
		TokensIn:     completion.Usage.PromptTokens,
		TokensOut:    completion.Usage.CompletionTokens,
		CostUSD:      0, // OpenAI does not return cost in the API response
		ModelUsed:    string(completion.Model),
		DurationMs:   durationMs,
		SessionID:    opts.SessionID,
	}, nil
}
