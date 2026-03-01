package bridgeopenai

import (
	"github.com/orchestra-mcp/plugin-bridge-openai/internal"
	"github.com/orchestra-mcp/sdk-go/plugin"
)

// Register adds all OpenAI bridge tools to the builder.
func Register(builder *plugin.PluginBuilder) {
	bp := internal.NewBridgePlugin()
	bp.RegisterTools(builder)
}
