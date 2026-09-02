package herdr

import (
	"context"
	"fmt"
	"time"
)

// command.go is the `charly herdr` kong command tree. Targeting flags live on
// the Globals struct embedded in the ROOT node only — kong treats embedded-root
// flags as GLOBAL (parseable before OR after any subcommand) — and the root is
// kong.Bind()'d so every leaf Runner receives the populated root and reads the
// resolved targeting from it. Each leaf resolves the session target through the
// same ladder (session.go) before dialing the NDJSON socket API.

// Globals are the targeting flags every subcommand accepts (global flags).
type Globals struct {
	SessionName string `name:"session" help:"Named Herdr session to target (~/.config/herdr/sessions/<name>/herdr.sock)"`
	Endpoint    string `name:"endpoint" help:"TCP endpoint (host:port) speaking the herdr NDJSON protocol (e.g. the candy socket bridge)"`
	Focused     bool   `name:"focused" help:"Explicitly target the focused Herdr session (off-limits by default; HERDR_ENV=1 lifts the guard)"`
}

// HerdrCmd is the `charly herdr` command tree.
type HerdrCmd struct {
	Globals

	Status    StatusCmd    `cmd:"" help:"Ping the target and summarize session state"`
	Config    ConfigCmd    `cmd:"" help:"Show the resolved session target"`
	Session   SessionCmd   `cmd:"" help:"Session helpers (snapshot)"`
	Workspace WorkspaceCmd `cmd:"" help:"Workspace helpers (list, create)"`
	Tab       TabCmd       `cmd:"" help:"Tab helpers (list, create)"`
	Pane      PaneCmd      `cmd:"" help:"Pane helpers (list, split, run, read, send-text, send-keys, wait-output)"`
	Agent     AgentCmd     `cmd:"" help:"Agent helpers (list, get, wait, prompt, report)"`
}

// ---------------------------------------------------------------------------
// Targeting + engine plumbing shared by every leaf.
// ---------------------------------------------------------------------------

func (g Globals) opts() targetOpts {
	return targetOpts{Session: g.SessionName, Endpoint: g.Endpoint, Focused: g.Focused}
}

// leafRun resolves the session target, dials the client, and runs the leaf's
// engine function.
func leafRun(g Globals, fn func(context.Context, *engine) (string, error)) error {
	target, err := resolveTarget(g.opts())
	if err != nil {
		return err
	}
	ctx := context.Background()
	out, err := fn(ctx, &engine{client: newNDJSONClient(target.Dial), target: target})
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}

// withTimeout applies a bounded context for waits.
func withTimeout(ctx context.Context, ms int64) (context.Context, context.CancelFunc) {
	if ms <= 0 {
		ms = 120000
	}
	return context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
}

// ---------------------------------------------------------------------------
// status / config
// ---------------------------------------------------------------------------

type StatusCmd struct{}

func (c StatusCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.status(ctx)
	})
}

type ConfigCmd struct{}

func (c ConfigCmd) Run(root *HerdrCmd) error {
	target, err := resolveTarget(root.opts())
	if err != nil {
		return err
	}
	fmt.Println("target:  " + target.String())
	fmt.Println("source:  " + target.Source)
	fmt.Println("focused: " + boolStr(target.Focused))
	return nil
}

func boolStr(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ---------------------------------------------------------------------------
// session
// ---------------------------------------------------------------------------

type SessionCmd struct {
	Snapshot SessionSnapshotCmd `cmd:"" help:"Print the live session snapshot (JSON)"`
}

type SessionSnapshotCmd struct{}

func (c SessionSnapshotCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.sessionSnapshot(ctx)
	})
}

// ---------------------------------------------------------------------------
// workspace
// ---------------------------------------------------------------------------

type WorkspaceCmd struct {
	List   WorkspaceListCmd   `cmd:"" help:"List workspaces"`
	Create WorkspaceCreateCmd `cmd:"" help:"Create a workspace"`
}

type WorkspaceListCmd struct{}

func (c WorkspaceListCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) { return eng.workspaces(ctx) })
}

type WorkspaceCreateCmd struct {
	Label string `name:"label" help:"Workspace label"`
	Cwd   string `name:"cwd" help:"Working directory for the initial pane"`
	Focus bool   `name:"focus" help:"Focus the new workspace"`
}

func (c WorkspaceCreateCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.workspaceCreate(ctx, c.Label, c.Cwd, c.Focus)
	})
}

// ---------------------------------------------------------------------------
// tab
// ---------------------------------------------------------------------------

type TabCmd struct {
	List   TabListCmd   `cmd:"" help:"List tabs"`
	Create TabCreateCmd `cmd:"" help:"Create a tab"`
}

type TabListCmd struct {
	Workspace string `name:"workspace" help:"Workspace ID filter"`
}

func (c TabListCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.tabs(ctx, c.Workspace)
	})
}

type TabCreateCmd struct {
	Workspace string `name:"workspace" help:"Workspace ID to create the tab in"`
	Label     string `name:"label" help:"Tab label"`
	Focus     bool   `name:"focus" help:"Focus the new tab"`
}

func (c TabCreateCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.tabCreate(ctx, c.Workspace, c.Label, c.Focus)
	})
}

// ---------------------------------------------------------------------------
// pane
// ---------------------------------------------------------------------------

type PaneCmd struct {
	List       PaneListCmd       `cmd:"" help:"List panes"`
	Split      PaneSplitCmd      `cmd:"" help:"Split a pane"`
	Run        PaneRunCmd        `cmd:"" help:"Run a command in a pane (send text + Enter)"`
	Read       PaneReadCmd       `cmd:"" help:"Read pane terminal output"`
	SendText   PaneSendTextCmd   `cmd:"" help:"Send literal text to a pane (no Enter)"`
	SendKeys   PaneSendKeysCmd   `cmd:"" help:"Send logical key presses to a pane (esc, ctrl+c, ...)"`
	WaitOutput PaneWaitOutputCmd `cmd:"" help:"Wait for matching pane output"`
}

type PaneListCmd struct {
	Workspace string `name:"workspace" help:"Workspace ID filter"`
}

func (c PaneListCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.panes(ctx, c.Workspace)
	})
}

type PaneSplitCmd struct {
	Pane      string `name:"pane" help:"Pane ID to split (default: the focused pane)"`
	Direction string `name:"direction" default:"right" help:"Split direction (right, down)"`
	Cwd       string `name:"cwd" help:"Working directory for the new pane"`
	Focus     bool   `name:"focus" help:"Focus the new pane"`
}

func (c PaneSplitCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.paneSplit(ctx, c.Pane, c.Direction, c.Cwd, c.Focus)
	})
}

type PaneRunCmd struct {
	Pane    string   `arg:"" name:"pane" help:"Pane ID"`
	Command []string `arg:"" name:"command" help:"Command words (joined with spaces)"`
}

func (c PaneRunCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.paneRun(ctx, c.Pane, joinArgs(c.Command))
	})
}

type PaneReadCmd struct {
	Pane   string `arg:"" name:"pane" help:"Pane ID"`
	Source string `name:"source" default:"recent" help:"Terminal snapshot source (visible, recent, recent-unwrapped, detection)"`
	Lines  int    `name:"lines" help:"Number of lines to read"`
}

func (c PaneReadCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.paneRead(ctx, c.Pane, c.Source, c.Lines)
	})
}

type PaneSendTextCmd struct {
	Pane string `arg:"" name:"pane" help:"Pane ID"`
	Text string `arg:"" name:"text" help:"Literal text"`
}

func (c PaneSendTextCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.paneSendText(ctx, c.Pane, c.Text)
	})
}

type PaneSendKeysCmd struct {
	Pane string   `arg:"" name:"pane" help:"Pane ID"`
	Keys []string `arg:"" name:"keys" help:"Logical key names (esc, ctrl+c, ...)"`
}

func (c PaneSendKeysCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.paneSendKeys(ctx, c.Pane, c.Keys)
	})
}

type PaneWaitOutputCmd struct {
	Pane    string `arg:"" name:"pane" help:"Pane ID"`
	Match   string `name:"match" help:"Match a literal substring"`
	Regex   string `name:"regex" help:"Match a Rust regular expression"`
	Source  string `name:"source" default:"recent" help:"Terminal snapshot source (visible, recent, recent-unwrapped)"`
	Timeout int64  `name:"timeout-ms" default:"120000" help:"Fail after this many milliseconds"`
}

func (c PaneWaitOutputCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		ctx2, cancel := withTimeout(ctx, c.Timeout)
		defer cancel()
		return eng.paneWaitOutput(ctx2, c.Pane, c.Match, c.Regex, c.Source, c.Timeout)
	})
}

// ---------------------------------------------------------------------------
// agent
// ---------------------------------------------------------------------------

type AgentCmd struct {
	List   AgentListCmd   `cmd:"" help:"List agents"`
	Get    AgentGetCmd    `cmd:"" help:"Show an agent"`
	Wait   AgentWaitCmd   `cmd:"" help:"Wait until an agent reaches a lifecycle state"`
	Prompt AgentPromptCmd `cmd:"" help:"Submit a prompt to an agent"`
	Report AgentReportCmd `cmd:"" help:"Report pane agent lifecycle state (pane.report_agent)"`
}

type AgentListCmd struct{}

func (c AgentListCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) { return eng.agents(ctx) })
}

type AgentGetCmd struct {
	Target string `arg:"" name:"target" help:"Agent name or pane ID"`
}

func (c AgentGetCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.agentGet(ctx, c.Target)
	})
}

type AgentWaitCmd struct {
	Target  string `arg:"" name:"target" help:"Agent name or pane ID"`
	Until   string `name:"until" help:"State to match (idle, working, blocked, done, unknown); default idle, done, blocked"`
	Timeout int64  `name:"timeout-ms" default:"120000" help:"Fail after this many milliseconds"`
}

func (c AgentWaitCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		ctx2, cancel := withTimeout(ctx, c.Timeout)
		defer cancel()
		return eng.agentWait(ctx2, c.Target, c.Until, c.Timeout)
	})
}

type AgentPromptCmd struct {
	Target  string `arg:"" name:"target" help:"Agent name or pane ID"`
	Text    string `arg:"" name:"text" help:"Prompt text"`
	Wait    bool   `name:"wait" help:"Wait for the first settled lifecycle state after submission"`
	Timeout int64  `name:"timeout-ms" default:"120000" help:"Fail after this many milliseconds"`
}

func (c AgentPromptCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		ctx2, cancel := withTimeout(ctx, c.Timeout)
		defer cancel()
		return eng.agentPrompt(ctx2, c.Target, c.Text, c.Wait, c.Timeout)
	})
}

type AgentReportCmd struct {
	Pane    string `arg:"" name:"pane" help:"Pane ID"`
	Agent   string `arg:"" name:"agent" help:"Agent label"`
	Source  string `name:"source" default:"charly" help:"Agent source/integration id"`
	State   string `name:"state" required:"" help:"Lifecycle state (idle, working, blocked, unknown)"`
	Message string `name:"message" help:"Optional lifecycle message"`
}

func (c AgentReportCmd) Run(root *HerdrCmd) error {
	return leafRun(root.Globals, func(ctx context.Context, eng *engine) (string, error) {
		return eng.agentReport(ctx, c.Pane, c.Source, c.Agent, c.State, c.Message)
	})
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

func joinArgs(words []string) string {
	s := ""
	for i, w := range words {
		if i > 0 {
			s += " "
		}
		s += w
	}
	return s
}
