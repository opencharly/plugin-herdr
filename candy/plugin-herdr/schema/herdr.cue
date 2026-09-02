// This compiled-in plugin's OWN CUE schema, served over the Describe channel — the
// typed plugin_input for the `herdr` check verb. It is the SINGLE SOURCE for this
// plugin's verb params, used two ways (the same contract core `spec` uses):
//
//  1. GENERATE the Go param struct — `cue exp gengotypes` emits
//     ../params/cue_types_gen.go, so the provider decodes plugin_input into a TYPED
//     struct, never a hand-parsed map.
//  2. VALIDATE authored input AT RUNTIME — the plugin serves this source over the
//     Describe channel; the host splices it onto the base (base ++ plugin) and
//     validates every authored `herdr:` step's plugin_input against #HerdrInput.
//
// The verb is HOST-BASED (the mcp pattern, mirroring plugin-agentteams): the
// provider resolves the in-venue herdr socket bridge port to a host-routable
// address over the reverse channel (cc.ResolveEndpoint) and speaks the SAME NDJSON
// socket protocol the `charly herdr` command plugin uses (R3 — one protocol
// surface covers the CLI and every bed).
//
// SELF-CONTAINED: it references NO base def, so it compiles standalone (the SDK's
// serve-side check + gengotypes) AND splices onto the base (base ++ plugin is a
// def-name collision check, not a base-reference resolver).

// #HerdrInput is the `herdr` verb's plugin_input: the method name plus its
// method-exclusive modifiers.
#HerdrInput: {
	// method — the herdr verb method name (the verb's PRIMARY input field, so
	// `herdr: ping` desugars to {method: "ping"}).
	method: ("ping" | "session-snapshot" | "workspace-list" | "tab-list" | "pane-list" | "agent-list" | "pane-wait-output" | "agent-wait" | "agent-prompt") @go(Method,type=string)
	// workspace — workspace ID filter for tab-list / pane-list.
	workspace?: string
	// pane — the pane ID for pane-wait-output.
	pane?: string
	// agent — the agent name or pane ID for agent-wait / agent-prompt.
	agent?: string
	// text — the prompt text for agent-prompt.
	text?: string
	// match — literal substring for pane-wait-output (mutually exclusive with regex).
	match?: string
	// regex — Rust regular expression for pane-wait-output (mutually exclusive with match).
	regex?: string
	// source — terminal read source, CLI spelling (recent-unwrapped maps to the
	// wire's recent_unwrapped). Default: recent.
	source?: ("visible" | "recent" | "recent-unwrapped" | "detection") @go(Source,type=string)
	// until — one agent lifecycle state for agent-wait (defaults: idle, done, blocked).
	until?: ("idle" | "working" | "blocked" | "done" | "unknown") @go(Until,type=string)
}
