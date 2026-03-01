package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
	"github.com/orchestra-mcp/sdk-go/helpers"
	"github.com/orchestra-mcp/sdk-go/plugin"
	"google.golang.org/protobuf/types/known/structpb"
)

// --- spawn_session ---

// SpawnSessionSchema returns the JSON Schema for the spawn_session tool.
func SpawnSessionSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "The prompt to send",
			},
			"resume": map[string]any{
				"type":        "boolean",
				"description": "Resume existing session with conversation history (default: false)",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Model to use (e.g., gpt-4o, gpt-4o-mini, o1, o3-mini)",
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
		"required": []any{"session_id", "prompt"},
	})
	return s
}

// SpawnSession returns a tool handler that creates or resumes a persistent
// OpenAI API session. When resume=true and a session with the given ID already
// exists, the full conversation history is replayed so the model has context.
func SpawnSession(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id", "prompt"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")
		resume := helpers.GetBool(req.Arguments, "resume")

		opts, err := parseCommonOpts(req.Arguments)
		if err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}
		opts.SessionID = sessionID

		// If resuming, load conversation history from existing session.
		var history []HistoryMessage
		if resume {
			existing := bridge.Plugin.GetSession(sessionID)
			if existing != nil {
				history = existing.History
				// Preserve model from existing session if not overridden.
				if opts.Model == "" && existing.Model != "" {
					opts.Model = existing.Model
				}
			}
		}
		opts.History = history

		// Call the OpenAI API synchronously.
		resp, err := bridge.Prompt(ctx, opts)
		if err != nil {
			return helpers.ErrorResult("api_error", err.Error()), nil
		}

		// Update the session with the new conversation turn.
		existing := bridge.Plugin.GetSession(sessionID)
		if existing == nil {
			existing = &Session{
				SessionID: sessionID,
				Model:     resp.ModelUsed,
				StartedAt: time.Now().Format(time.RFC3339),
				History:   []HistoryMessage{},
			}
		}
		// Append this turn to history.
		existing.History = append(existing.History,
			HistoryMessage{Role: "user", Content: opts.Prompt},
			HistoryMessage{Role: "assistant", Content: resp.ResponseText},
		)
		existing.Model = resp.ModelUsed
		existing.LastResp = resp

		bridge.Plugin.TrackSession(existing)

		return helpers.TextResult(formatChatResponse(resp)), nil
	}
}

// --- kill_session ---

// KillSessionSchema returns the JSON Schema for the kill_session tool.
func KillSessionSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID to remove",
			},
		},
		"required": []any{"session_id"},
	})
	return s
}

// KillSession returns a tool handler that removes a session and its
// conversation history from the active sessions map.
func KillSession(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")

		session := bridge.Plugin.RemoveSession(sessionID)
		if session == nil {
			return helpers.ErrorResult("not_found",
				fmt.Sprintf("no active session found with ID %q", sessionID)), nil
		}

		return helpers.TextResult(
			fmt.Sprintf("Removed session **%s** (%d messages in history)",
				sessionID, len(session.History))), nil
	}
}

// --- session_status ---

// SessionStatusSchema returns the JSON Schema for the session_status tool.
func SessionStatusSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type": "object",
		"properties": map[string]any{
			"session_id": map[string]any{
				"type":        "string",
				"description": "Session UUID to check",
			},
		},
		"required": []any{"session_id"},
	})
	return s
}

// SessionStatus returns a tool handler that reports the current status of an
// OpenAI API session, including conversation length and last response metadata.
func SessionStatus(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		if err := helpers.ValidateRequired(req.Arguments, "session_id"); err != nil {
			return helpers.ErrorResult("validation_error", err.Error()), nil
		}

		sessionID := helpers.GetString(req.Arguments, "session_id")

		session := bridge.Plugin.GetSession(sessionID)
		if session == nil {
			return helpers.ErrorResult("not_found",
				fmt.Sprintf("no session found with ID %q", sessionID)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Session: %s\n\n", sessionID)
		fmt.Fprintf(&b, "- **Status:** active\n")
		fmt.Fprintf(&b, "- **Model:** %s\n", session.Model)
		fmt.Fprintf(&b, "- **Started:** %s\n", session.StartedAt)
		fmt.Fprintf(&b, "- **Messages:** %d\n", len(session.History))

		// If there is a last response, include its metadata.
		if resp := session.LastResp; resp != nil {
			b.WriteString("\n### Last Response\n\n")
			b.WriteString(formatChatResponse(resp))
		}

		return helpers.TextResult(b.String()), nil
	}
}

// --- list_active ---

// ListActiveSchema returns the JSON Schema for the list_active tool.
func ListActiveSchema() *structpb.Struct {
	s, _ := structpb.NewStruct(map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	})
	return s
}

// ListActive returns a tool handler that lists all tracked OpenAI API sessions
// with their current status.
func ListActive(bridge *Bridge) plugin.ToolHandler {
	return func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error) {
		sessions := bridge.Plugin.ListSessions()

		if len(sessions) == 0 {
			return helpers.TextResult("## Active Sessions\n\nNo active sessions.\n"), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "## Active Sessions (%d)\n\n", len(sessions))
		fmt.Fprintf(&b, "| Session ID | Model | Messages | Started |\n")
		fmt.Fprintf(&b, "|------------|-------|----------|---------|\n")

		for _, s := range sessions {
			fmt.Fprintf(&b, "| %s | %s | %d | %s |\n",
				s.SessionID, s.Model, len(s.History), s.StartedAt)
		}

		return helpers.TextResult(b.String()), nil
	}
}
