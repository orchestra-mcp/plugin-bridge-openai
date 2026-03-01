// Package tools contains the tool schemas and handler functions for the
// bridge.openai plugin. Each exported function pair (Schema + Handler) follows
// the same pattern used across all Orchestra plugins.
package tools

import (
	"context"
	"fmt"
	"strings"

	pluginv1 "github.com/orchestra-mcp/gen-go/orchestra/plugin/v1"
)

// ToolHandler is an alias for readability.
type ToolHandler = func(ctx context.Context, req *pluginv1.ToolRequest) (*pluginv1.ToolResponse, error)

// BridgePluginInterface defines the methods the tools package needs from the
// BridgePlugin. This avoids a circular import between internal and tools.
type BridgePluginInterface interface {
	TrackSession(session *Session)
	GetSession(sessionID string) *Session
	RemoveSession(sessionID string) *Session
	ListSessions() []*Session
}

// Session represents a conversation session with the OpenAI API.
// Unlike bridge-claude which manages OS processes, bridge-openai maintains
// conversation history so that resume=true replays the full history.
type Session struct {
	SessionID string
	Model     string
	StartedAt string
	History   []HistoryMessage
	LastResp  *ChatResponse
}

// HistoryMessage represents a single message in a conversation.
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PromptFunc is the function signature for sending a prompt to the OpenAI API.
// This is injected to avoid circular imports.
type PromptFunc func(ctx context.Context, opts SpawnOptions) (*ChatResponse, error)

// SpawnOptions mirrors the internal SpawnOptions for use by tool handlers.
type SpawnOptions struct {
	SessionID      string
	Resume         bool
	Prompt         string
	Model          string
	Workspace      string
	AllowedTools   []string
	PermissionMode string
	MaxBudget      float64
	SystemPrompt   string
	Env            map[string]string
	History        []HistoryMessage
}

// ChatResponse holds the result of a completed OpenAI API call.
type ChatResponse struct {
	ResponseText string  `json:"response_text"`
	TokensIn     int64   `json:"tokens_in"`
	TokensOut    int64   `json:"tokens_out"`
	CostUSD      float64 `json:"cost_usd"`
	ModelUsed    string  `json:"model_used"`
	DurationMs   int64   `json:"duration_ms"`
	SessionID    string  `json:"session_id"`
}

// Bridge holds the injected dependencies that tool handlers need.
type Bridge struct {
	Prompt PromptFunc
	Plugin BridgePluginInterface
}

// --- Common helpers ---

// formatChatResponse formats a ChatResponse as a Markdown string for display.
func formatChatResponse(resp *ChatResponse) string {
	var b strings.Builder
	b.WriteString(resp.ResponseText)
	b.WriteString("\n\n---\n")

	if resp.SessionID != "" {
		fmt.Fprintf(&b, "- **Session:** %s\n", resp.SessionID)
	}
	if resp.ModelUsed != "" {
		fmt.Fprintf(&b, "- **Model:** %s\n", resp.ModelUsed)
	}
	if resp.TokensIn > 0 || resp.TokensOut > 0 {
		fmt.Fprintf(&b, "- **Tokens:** %d in / %d out\n", resp.TokensIn, resp.TokensOut)
	}
	if resp.CostUSD > 0 {
		fmt.Fprintf(&b, "- **Cost:** $%.4f\n", resp.CostUSD)
	}
	if resp.DurationMs > 0 {
		fmt.Fprintf(&b, "- **Duration:** %dms\n", resp.DurationMs)
	}

	return b.String()
}
