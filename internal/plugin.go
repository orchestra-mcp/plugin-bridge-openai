package internal

import (
	"context"
	"sync"

	"github.com/orchestra-mcp/sdk-go/plugin"
	"github.com/orchestra-mcp/plugin-bridge-openai/internal/tools"
)

// BridgePlugin manages OpenAI API sessions and registers all bridge tools.
type BridgePlugin struct {
	sessions map[string]*tools.Session
	mu       sync.RWMutex
}

// NewBridgePlugin creates a new BridgePlugin with an empty session map.
func NewBridgePlugin() *BridgePlugin {
	return &BridgePlugin{
		sessions: make(map[string]*tools.Session),
	}
}

// RegisterTools registers all 5 bridge tools with the plugin builder.
func (bp *BridgePlugin) RegisterTools(builder *plugin.PluginBuilder) {
	bridge := &tools.Bridge{
		Prompt: bp.promptAdapter,
		Plugin: bp,
	}

	// --- Prompt tool (1) ---
	builder.RegisterTool("ai_prompt",
		"Send a one-shot prompt to the OpenAI API and return the response",
		tools.AIPromptSchema(), tools.AIPrompt(bridge))

	// --- Session tools (4) ---
	builder.RegisterTool("spawn_session",
		"Create or resume a persistent OpenAI API session with conversation history",
		tools.SpawnSessionSchema(), tools.SpawnSession(bridge))

	builder.RegisterTool("kill_session",
		"Remove an OpenAI API session and its conversation history",
		tools.KillSessionSchema(), tools.KillSession(bridge))

	builder.RegisterTool("session_status",
		"Check the status of an OpenAI API session",
		tools.SessionStatusSchema(), tools.SessionStatus(bridge))

	builder.RegisterTool("list_active",
		"List all active OpenAI API sessions",
		tools.ListActiveSchema(), tools.ListActive(bridge))
}

// promptAdapter calls the OpenAI API via the internal client.
func (bp *BridgePlugin) promptAdapter(ctx context.Context, opts tools.SpawnOptions) (*tools.ChatResponse, error) {
	return CallOpenAI(ctx, opts)
}

// --- BridgePluginInterface implementation ---

// TrackSession adds or updates a session in the active map.
func (bp *BridgePlugin) TrackSession(session *tools.Session) {
	if session == nil {
		return
	}
	bp.mu.Lock()
	defer bp.mu.Unlock()
	bp.sessions[session.SessionID] = session
}

// GetSession returns the session for the given ID, or nil if not found.
func (bp *BridgePlugin) GetSession(sessionID string) *tools.Session {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	session, ok := bp.sessions[sessionID]
	if !ok {
		return nil
	}
	return session
}

// RemoveSession removes and returns the session for the given ID.
func (bp *BridgePlugin) RemoveSession(sessionID string) *tools.Session {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	session, ok := bp.sessions[sessionID]
	if !ok {
		return nil
	}
	delete(bp.sessions, sessionID)
	return session
}

// ListSessions returns a snapshot of all active sessions.
func (bp *BridgePlugin) ListSessions() []*tools.Session {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	sessions := make([]*tools.Session, 0, len(bp.sessions))
	for _, s := range bp.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// ClearAll removes all sessions. Called during shutdown.
func (bp *BridgePlugin) ClearAll() {
	bp.mu.Lock()
	defer bp.mu.Unlock()
	for id := range bp.sessions {
		delete(bp.sessions, id)
	}
}
