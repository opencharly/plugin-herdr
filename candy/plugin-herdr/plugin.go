// Package herdr is the importable form of the charly `herdr` plugin: a
// COMPILED-IN `command:herdr` CLI for a Herdr terminal-multiplexer session PLUS
// the `herdr:` check VERB (the declarative controller-probe counterpart, verb.go).
// A command provider dispatches via the pb Invoke(OpRun) envelope — decode the
// pass-through `{"args":[...]}` and kong-parse them into the HerdrCmd tree
// (sdk.RunInProcCLI), so the handler runs in charly's OWN process with native
// stdio/TTY. The verb provider dispatches via Invoke with the full #Op as
// params_json (the mcp pattern): it resolves the in-venue herdr socket bridge
// port to a host-routable address over the reverse channel and probes with the
// SAME NDJSON socket client the command uses (R3 — one protocol surface covers
// the CLI and every bed). Usable COMPILED-IN (NewProvider()/NewMeta() via
// plugins_generated.go) OR served OUT-OF-PROCESS by the cmd/serve shim — both
// placements run the SAME runCommand / runVerbHerdr (placement-invisible, F8).
package herdr

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alecthomas/kong"

	"github.com/opencharly/plugin-herdr/candy/plugin-herdr/params"
	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/kit"
	pb "github.com/opencharly/spec/proto"
	"github.com/opencharly/spec/spec"
)

const calver = "2026.245.1000"

//go:embed schema/*.cue
var schemaFS embed.FS

// NewProvider returns the provider for in-proc registration (compiled-in) or
// out-of-proc serving.
func NewProvider() pb.ProviderServer { return &provider{} }

// NewMeta advertises command:herdr + verb:herdr via a lazy Describe: the kong
// CLIModel is reflected INSIDE Describe (herdrMeta) rather than eagerly in the
// constructor — a kong reflection regression then surfaces as a Describe error
// at plugin registration, loud but never a panic crashing every charly startup
// (the plugin-agent pattern). The verb capability carries the #HerdrInput def
// served from the plugin's own schema/*.cue; the command capability carries no
// InputDef — a command's args are pass-through CLI tokens, not a structured
// plugin_input.
func NewMeta() pb.PluginMetaServer { return herdrMeta{} }

// herdrMeta is the plugin's PluginMetaServer: NewMeta stays trivial (it is
// called at process init by plugins_generated.go) and all fallible reflection
// happens in Describe, which can return an error.
type herdrMeta struct {
	pb.UnimplementedPluginMetaServer
}

func (herdrMeta) Describe(context.Context, *pb.Empty) (*pb.Capabilities, error) {
	model, err := commandModel()
	if err != nil {
		return nil, err
	}
	return sdk.BuildCapabilities(calver,
		[]sdk.ProvidedCapability{
			{Class: "command", Word: "herdr", CommandModel: model},
			{Class: "verb", Word: "herdr", InputDef: "#HerdrInput", Primary: "method"},
		},
		schemaFS, "schema")
}

type provider struct{ pb.UnimplementedProviderServer }

// Invoke dispatches one operation for the plugin's capabilities. A "command" op
// runs the pass-through CLI args in charly's own process (OpRun); a "verb" op
// runs one `herdr:` check step (the full #Op as params_json + a CheckEnv
// snapshot as env — the mcp pattern). (Out-of-process command dispatch is
// fork/exec → CliMain, never this gRPC path.)
func (provider) Invoke(ctx context.Context, req *pb.InvokeRequest) (*pb.InvokeReply, error) {
	if req.GetClass() == "command" {
		return invokeCommand(req)
	}
	if req.GetClass() == "verb" {
		return invokeVerb(ctx, req)
	}
	return nil, fmt.Errorf("herdr: unsupported class %q", req.GetClass())
}

// Reserved implements spec.CheckVerbProvider: the verb word.
func (p *provider) Reserved() string { return "herdr" }

// RunVerb implements spec.CheckVerbProvider — the COMPILED-IN verb dispatch. The
// host recognizes a compiled-in pb.ProviderServer that ALSO implements this typed
// contract (hostVerbResolver.RunVerb) and threads the live host CheckContext in
// (hostCheckContext) — the executor-bearing surface a host-coupled verb needs
// (ResolveEndpoint + Exec). The out-of-process placement runs the SAME core via
// invokeVerb (the pb Invoke envelope with the broker attached) — placement-
// invisible, F8.
func (p *provider) RunVerb(ctx context.Context, cc spec.CheckContext, op *spec.Op) spec.CheckVerbResult {
	var in params.HerdrInput
	kit.DecodeInput(op.PluginInput, &in)
	method := in.Method
	if cc.Mode() == spec.CheckModeBox {
		return spec.CheckVerbResult{Status: spec.StatusSkip, Message: fmt.Sprintf("herdr: %s requires a live herdr venue (skip under charly check box)", method)}
	}
	out, runErr := runVerbHerdr(ctx, cc, op, in)
	return verbVerdict(method, out, runErr, op)
}

// verbVerdict grades the verb's output against the authored op matchers
// (exit_status / stdout / stderr) and returns the typed verdict — the SAME shared
// pipeline the out-of-process path runs (sdk.VerbVerdict), converted from the wire
// form to the typed spec.CheckVerbResult a compiled-in RunVerb returns (R3).
func verbVerdict(method, out string, runErr error, op *spec.Op) spec.CheckVerbResult {
	reply, err := sdk.VerbVerdict("herdr", method, out, runErr, op, false)
	if err != nil {
		return spec.CheckVerbResult{Status: spec.StatusFail, Message: err.Error()}
	}
	var wire struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(reply.ResultJson, &wire); err != nil {
		return spec.CheckVerbResult{Status: spec.StatusFail, Message: err.Error()}
	}
	status := spec.StatusFail
	switch wire.Status {
	case "pass":
		status = spec.StatusPass
	case "skip":
		status = spec.StatusSkip
	}
	return spec.CheckVerbResult{Status: status, Message: wire.Message}
}

// invokeCommand handles OpRun for the COMPILED-IN (in-proc) dispatch: decode the
// pass-through {args} and run the command effect in charly's own process.
func invokeCommand(req *pb.InvokeRequest) (*pb.InvokeReply, error) {
	if req.GetOp() != sdk.OpRun {
		return nil, fmt.Errorf("herdr: unsupported op %q (only %q)", req.GetOp(), sdk.OpRun)
	}
	var input struct {
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(req.GetParamsJson(), &input); err != nil {
		return nil, fmt.Errorf("herdr: decode args: %w", err)
	}
	if err := runCommand(input.Args); err != nil {
		return nil, err
	}
	return &pb.InvokeReply{}, nil
}

// herdrEnv is the plugin-side decode of the CheckEnv the host ships as
// Operation.Env for a `herdr:` check step — only Mode matters here (the verb
// probes a live venue socket, never a container).
type herdrEnv struct {
	Box  string `json:"box"`
	Mode string `json:"mode"` // "live" | "box"
}

// invokeVerb runs one `herdr:` check operation (the out-of-process placement).
func invokeVerb(ctx context.Context, req *pb.InvokeRequest) (*pb.InvokeReply, error) {
	var op spec.Op
	if len(req.GetParamsJson()) > 0 {
		if err := json.Unmarshal(req.GetParamsJson(), &op); err != nil {
			return sdk.ResultJSON("fail", "herdr: decode op: "+err.Error())
		}
	}
	var in params.HerdrInput
	kit.DecodeInput(op.PluginInput, &in)
	var env herdrEnv
	if len(req.GetEnvJson()) > 0 {
		_ = json.Unmarshal(req.GetEnvJson(), &env)
	}
	method := in.Method
	if env.Mode == "box" {
		return sdk.ResultJSON("skip", fmt.Sprintf("herdr: %s requires a live herdr venue (skip under charly check box)", method))
	}
	cc, err := sdk.NewCheckContext(req.GetExecutorBrokerId(), req.GetEnvJson())
	if err != nil {
		return sdk.ResultJSON("fail", fmt.Sprintf("herdr: %s: %v", method, err))
	}
	out, runErr := runVerbHerdr(ctx, cc, &op, in)
	return sdk.VerbVerdict("herdr", method, out, runErr, &op, false)
}

// CliMain is the OUT-OF-PROCESS CLI-mode entry (charly fork/execs the binary with
// the pass-through tokens after `charly herdr`). It runs the SAME effect as the
// in-proc Invoke(OpRun) path.
func CliMain(args []string) int {
	if err := runCommand(args); err != nil {
		fmt.Fprintf(os.Stderr, "charly herdr: %v\n", err)
		return 1
	}
	return 0
}

// runCommand parses the pass-through args of a COMPILED-IN command — which runs
// in charly's OWN process — so it must NEVER let kong terminate the host: kong's
// default Exit is os.Exit, and a raw kong.New/Parse would make `charly herdr
// --help` kill charly whole. sdk.RunInProcCLI is the house in-proc helper
// (sdk/clidispatch.go documents the hazard).
func runCommand(args []string) error {
	var command HerdrCmd
	return sdk.RunInProcCLI("herdr", &command, args,
		kong.Description("Manage a Herdr terminal-multiplexer session: workspace/tab/pane/agent helpers over the herdr NDJSON socket API"),
		kong.Bind(&command))
}

// commandModel reflects the kong grammar into a CLIModel. Every error propagates
// to Describe (no panic).
func commandModel() (*spec.CLIModel, error) {
	return sdk.BuildCLIModel(&HerdrCmd{}, "herdr", calver, "herdr")
}
