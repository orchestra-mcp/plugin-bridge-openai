// Command bridge-openai is the entry point for the bridge.openai plugin
// binary. It calls the OpenAI Chat Completions API. This plugin does NOT
// require storage -- it manages in-memory session state only.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/orchestra-mcp/sdk-go/plugin"
	"github.com/orchestra-mcp/plugin-bridge-openai/internal"
)

func main() {
	builder := plugin.New("bridge.openai").
		Version("0.1.0").
		Description("OpenAI API bridge — sends prompts to OpenAI-compatible APIs").
		Author("Orchestra").
		Binary("bridge-openai").
		ProvidesAI("openai", "grok", "perplexity", "deepseek", "qwen", "kimi")

	bp := internal.NewBridgePlugin()
	bp.RegisterTools(builder)

	p := builder.BuildWithTools()
	p.ParseFlags()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		bp.ClearAll() // clear all session state
		cancel()
	}()

	if err := p.Run(ctx); err != nil {
		log.Fatalf("bridge.openai: %v", err)
	}
}
