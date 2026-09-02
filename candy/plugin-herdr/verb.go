package herdr

import (
	"context"
	"fmt"
	"time"

	"github.com/opencharly/plugin-herdr/candy/plugin-herdr/params"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// verb.go is the `herdr:` check VERB — the declarative counter-party of the
// `charly herdr` command plugin. It is HOST-BASED (the mcp pattern, mirroring
// plugin-agentteams): the provider resolves the in-venue herdr socket-bridge
// port to a host-routable address over the reverse channel
// (cc.ResolveEndpoint — the pod's published bridge port), then dials the herdr
// NDJSON socket protocol with the SAME client the command plugin uses (R3 —
// one protocol surface covers the CLI and every bed).

// herdrVenuePort is the fixed in-venue port the herdr candy's socket bridge
// (socat: TCP accept → the herdr server unix socket) listens on. The verb and
// the box agree on this number; the candy's service unit owns the actual bridge.
const herdrVenuePort = 8095

// defaultVerbTimeout is the fallback when a `herdr:` step carries no timeout.
const defaultVerbTimeout = 120 * time.Second

// runVerbHerdr is the entry the provider calls: resolve the venue bridge over
// the reverse channel, then dispatch.
func runVerbHerdr(ctx context.Context, cc kit.CheckContext, op *spec.Op, in params.HerdrInput) (string, error) {
	return runVerbHerdrResolved(ctx, func(c context.Context, port int) (string, error) {
		return cc.ResolveEndpoint(c, port)
	}, op, in)
}

// runVerbHerdrResolved is the dispatch with the venue-endpoint resolution
// injected, so unit tests can point the verb at a local fake herdr server.
func runVerbHerdrResolved(ctx context.Context, resolve func(context.Context, int) (string, error), op *spec.Op, in params.HerdrInput) (string, error) {
	addr, err := resolve(ctx, herdrVenuePort)
	if err != nil {
		return "", fmt.Errorf("resolve herdr venue endpoint: %w", err)
	}
	if addr == "" {
		return "", fmt.Errorf("no live herdr venue for the herdr verb (box-mode or no socket bridge)")
	}
	eng := &engine{client: newNDJSONClient("tcp://" + addr), target: sessionTarget{Dial: "tcp://" + addr, Kind: "tcp", Addr: addr, Source: "venue-bridge"}}

	timeoutMs := int64(parseTimeout(op.Timeout, defaultVerbTimeout).Milliseconds())
	switch in.Method {
	case "ping":
		return eng.ping(ctx)
	case "session-snapshot":
		return eng.summarise(ctx)
	case "workspace-list":
		return eng.workspaces(ctx)
	case "tab-list":
		return eng.tabs(ctx, in.Workspace)
	case "pane-list":
		return eng.panes(ctx, in.Workspace)
	case "agent-list":
		return eng.agents(ctx)
	case "pane-wait-output":
		ctx2, cancel := context.WithTimeout(ctx, parseTimeout(op.Timeout, defaultVerbTimeout))
		defer cancel()
		return eng.paneWaitOutput(ctx2, in.Pane, in.Match, in.Regex, in.Source, timeoutMs)
	case "agent-wait":
		ctx2, cancel := context.WithTimeout(ctx, parseTimeout(op.Timeout, defaultVerbTimeout))
		defer cancel()
		return eng.agentWait(ctx2, in.Agent, in.Until, timeoutMs)
	case "agent-prompt":
		ctx2, cancel := context.WithTimeout(ctx, parseTimeout(op.Timeout, defaultVerbTimeout))
		defer cancel()
		return eng.agentPrompt(ctx2, in.Agent, in.Text, true, timeoutMs)
	default:
		return "", fmt.Errorf("unknown herdr method %q", in.Method)
	}
}
