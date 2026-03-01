package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/sdk-go/plugin"
	"google.golang.org/protobuf/types/known/structpb"
)

// --- ai_prompt ---

// AIPromptSchema returns the JSON Schema for the ai_prompt tool.
func AIPromptSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The prompt to send to the OpenAI API",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model to use (e.g., gpt-4o, gpt-4o-mini, o1, o3-mini)",
			},
			"workspace": map[string]any{
				"type":        "string",
				"description": "Working directory context (informational only)",
			},
			"system_prompt": map[string]any{
				"type":        "string",
				"description": "Custom system prompt",
			},
			"env": map[string]any{
				"type":        "string",
				"description": "JSON object of environment variables (e.g., {\"OPENAI_API_KEY\": \"sk-...\", \"OPENAI_BASE_URL\": \"https://...\"})",
			},
		},
		"required": []any{"prompt"},
	})
	return s
}

// AIPrompt returns a tool handler that sends a one-shot prompt to the OpenAI
// API and returns the response. This is always synchronous since API calls
// complete within seconds (no background process needed).
func AIPrompt(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "prompt"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		opts, err := parseCommonOpts(req.Arguments)
		if err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}
		// One-shot: no session ID, no history.
		opts.SessionID = ""
		opts.History = nil

		resp, err := bridge.Prompt(ctx, opts)
		if err != nil {
			return helpers.ErrorResult("api_error", err.Error()), nil
		}

		return helpers.TextResult(formatChatResponse(resp)), nil
	}
}

// --- Common helpers ---

// parseCommonOpts extracts the shared options from tool arguments.
func parseCommonOpts(args *structpb.Struct) (SpawnOptions, error) {
	prompt := helpers.GetString(args, "prompt")
	model := helpers.GetString(args, "model")
	workspace := helpers.GetString(args, "workspace")
	systemPrompt := helpers.GetString(args, "system_prompt")
	envRaw := helpers.GetString(args, "env")

	// Default workspace to current working directory.
	if workspace == "" {
		cwd, err := os.Getwd()
		if err == nil {
			workspace = cwd
		}
	}

	// Parse allowed tools from comma-separated string (for schema compat).
	allowedToolsRaw := helpers.GetString(args, "allowed_tools")
	var allowedTools []string
	if allowedToolsRaw != "" {
		for _, t := range strings.Split(allowedToolsRaw, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				allowedTools = append(allowedTools, t)
			}
		}
	}

	// Parse env from JSON string.
	var envMap map[string]string
	if envRaw != "" {
		if err := json.Unmarshal([]byte(envRaw), &envMap); err != nil {
			return SpawnOptions{}, fmt.Errorf("invalid env JSON: %w", err)
		}
	}

	return SpawnOptions{
		Prompt:       prompt,
		Model:        model,
		Workspace:    workspace,
		AllowedTools: allowedTools,
		SystemPrompt: systemPrompt,
		Env:          envMap,
	}, nil
}
