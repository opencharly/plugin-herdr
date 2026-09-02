package herdr

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// control.go is the plugin's engine: every `charly herdr` leaf and every
// `herdr:` verb method funnels through the SAME ndjsonClient + wire structs
// (R3 — one protocol surface, two placements). The result shapes mirror the
// herdr API schema (herdr api schema --json, protocol 20).

// ---------------------------------------------------------------------------
// Wire result shapes (success_response $defs)
// ---------------------------------------------------------------------------

type pongResult struct {
	Type     string `json:"type"`
	Version  string `json:"version"`
	Protocol int    `json:"protocol"`
}

type workspaceInfo struct {
	WorkspaceID string `json:"workspace_id"`
	Number      int    `json:"number"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	PaneCount   int    `json:"pane_count"`
	TabCount    int    `json:"tab_count"`
	ActiveTabID string `json:"active_tab_id"`
	AgentStatus string `json:"agent_status"`
}

type tabInfo struct {
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Number      int    `json:"number"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	PaneCount   int    `json:"pane_count"`
	AgentStatus string `json:"agent_status"`
}

type paneInfo struct {
	PaneID                string `json:"pane_id"`
	TerminalID            string `json:"terminal_id"`
	WorkspaceID           string `json:"workspace_id"`
	TabID                 string `json:"tab_id"`
	Focused               bool   `json:"focused"`
	AgentStatus           string `json:"agent_status"`
	Agent                 string `json:"agent"`
	Cwd                   string `json:"cwd"`
	ForegroundCwd         string `json:"foreground_cwd"`
	TerminalTitle         string `json:"terminal_title"`
	TerminalTitleStripped string `json:"terminal_title_stripped"`
	Label                 string `json:"label"`
}

type agentInfo struct {
	Name             string `json:"name"`
	Agent            string `json:"agent"`
	DisplayAgent     string `json:"display_agent"`
	AgentStatus      string `json:"agent_status"`
	Cwd              string `json:"cwd"`
	WorkspaceID      string `json:"workspace_id"`
	TabID            string `json:"tab_id"`
	PaneID           string `json:"pane_id"`
	Focused          bool   `json:"focused"`
	InteractiveReady bool   `json:"interactive_ready"`
	LaunchPending    bool   `json:"launch_pending"`
}

type paneReadResult struct {
	PaneID      string `json:"pane_id"`
	WorkspaceID string `json:"workspace_id"`
	TabID       string `json:"tab_id"`
	Source      string `json:"source"`
	Format      string `json:"format"`
	Text        string `json:"text"`
	Revision    int    `json:"revision"`
	Truncated   bool   `json:"truncated"`
}

type outputMatchedResult struct {
	Type        string         `json:"type"`
	PaneID      string         `json:"pane_id"`
	Revision    int            `json:"revision"`
	MatchedLine string         `json:"matched_line"`
	Read        paneReadResult `json:"read"`
}

type waitMatchedResult struct {
	Type  string          `json:"type"`
	Event json.RawMessage `json:"event"`
}

type sessionSnapshot struct {
	Version            string          `json:"version"`
	Protocol           int             `json:"protocol"`
	Workspaces         []workspaceInfo `json:"workspaces"`
	Tabs               []tabInfo       `json:"tabs"`
	Panes              []paneInfo      `json:"panes"`
	Agents             []agentInfo     `json:"agents"`
	FocusedWorkspaceID string          `json:"focused_workspace_id"`
	FocusedTabID       string          `json:"focused_tab_id"`
	FocusedPaneID      string          `json:"focused_pane_id"`
}

// ---------------------------------------------------------------------------
// engine
// ---------------------------------------------------------------------------

type engine struct {
	client *ndjsonClient
	target sessionTarget
}

// wireSource maps the CLI source spelling to the wire enum (SKILL + schema use
// recent_unwrapped on the wire, recent-unwrapped on the CLI).
func wireSource(cliSource, def string) string {
	switch cliSource {
	case "":
		return def
	case "recent-unwrapped":
		return "recent_unwrapped"
	default:
		return cliSource
	}
}

func (e *engine) ping(ctx context.Context) (string, error) {
	raw, arm, err := e.client.call(ctx, "ping", struct{}{})
	if err != nil {
		return "", err
	}
	pong, err := decodeResult[pongResult](raw, "pong", arm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pong (protocol %d, version %s)", pong.Protocol, pong.Version), nil
}

func (e *engine) status(ctx context.Context) (string, error) {
	pong, err := e.ping(ctx)
	if err != nil {
		return "", err
	}
	raw, arm, err := e.client.call(ctx, "session.snapshot", struct{}{})
	if err != nil {
		return "", err
	}
	wrapped, err := decodeResult[struct {
		Snapshot sessionSnapshot `json:"snapshot"`
	}](raw, "session_snapshot", arm)
	if err != nil {
		return "", err
	}
	snap := wrapped.Snapshot
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", pong)
	fmt.Fprintf(&b, "workspaces: %d · tabs: %d · panes: %d · agents: %d\n",
		len(snap.Workspaces), len(snap.Tabs), len(snap.Panes), len(snap.Agents))
	if snap.FocusedWorkspaceID != "" {
		label := snap.FocusedWorkspaceID
		for _, w := range snap.Workspaces {
			if w.WorkspaceID == snap.FocusedWorkspaceID && w.Label != "" {
				label = fmt.Sprintf("%s (%s)", w.WorkspaceID, w.Label)
			}
		}
		fmt.Fprintf(&b, "focused: workspace %s · tab %s · pane %s\n", label, snap.FocusedTabID, snap.FocusedPaneID)
	}
	return b.String(), nil
}

func (e *engine) sessionSnapshot(ctx context.Context) (string, error) {
	raw, _, err := e.client.call(ctx, "session.snapshot", struct{}{})
	if err != nil {
		return "", err
	}
	var pretty any
	if err := json.Unmarshal(raw, &pretty); err != nil {
		return "", err
	}
	out, err := json.MarshalIndent(pretty, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (e *engine) summarise(ctx context.Context) (string, error) {
	return e.status(ctx)
}

func (e *engine) workspaces(ctx context.Context) (string, error) {
	raw, arm, err := e.client.call(ctx, "workspace.list", struct{}{})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Workspaces []workspaceInfo `json:"workspaces"`
	}](raw, "workspace_list", arm)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, w := range res.Workspaces {
		focus := " "
		if w.Focused {
			focus = "*"
		}
		fmt.Fprintf(&b, "%s %s  %-20s  tabs=%d panes=%d  %s\n", focus, w.WorkspaceID, or(w.Label, ""), w.TabCount, w.PaneCount, w.AgentStatus)
	}
	b.WriteString(fmt.Sprintf("%d workspace(s)\n", len(res.Workspaces)))
	return b.String(), nil
}

func (e *engine) workspaceCreate(ctx context.Context, label, cwd string, focus bool) (string, error) {
	raw, arm, err := e.client.call(ctx, "workspace.create", map[string]any{
		"label": nullable(label), "cwd": nullable(cwd), "focus": focus,
	})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Workspace workspaceInfo `json:"workspace"`
		Tab       tabInfo       `json:"tab"`
		RootPane  paneInfo      `json:"root_pane"`
	}](raw, "workspace_created", arm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("workspace %s (%s) · tab %s · root pane %s\n",
		res.Workspace.WorkspaceID, or(res.Workspace.Label, ""), res.Tab.TabID, res.RootPane.PaneID), nil
}

func (e *engine) tabs(ctx context.Context, workspace string) (string, error) {
	raw, arm, err := e.client.call(ctx, "tab.list", map[string]any{"workspace_id": nullable(workspace)})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Tabs []tabInfo `json:"tabs"`
	}](raw, "tab_list", arm)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, t := range res.Tabs {
		focus := " "
		if t.Focused {
			focus = "*"
		}
		fmt.Fprintf(&b, "%s %s  %s  %-12s  panes=%d  %s\n", focus, t.TabID, t.WorkspaceID, or(t.Label, ""), t.PaneCount, t.AgentStatus)
	}
	b.WriteString(fmt.Sprintf("%d tab(s)\n", len(res.Tabs)))
	return b.String(), nil
}

func (e *engine) tabCreate(ctx context.Context, workspace, label string, focus bool) (string, error) {
	raw, arm, err := e.client.call(ctx, "tab.create", map[string]any{
		"workspace_id": nullable(workspace), "label": nullable(label), "focus": focus,
	})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Tab      tabInfo  `json:"tab"`
		RootPane paneInfo `json:"root_pane"`
	}](raw, "tab_created", arm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tab %s (%s) · root pane %s\n", res.Tab.TabID, or(res.Tab.Label, ""), res.RootPane.PaneID), nil
}

func (e *engine) panes(ctx context.Context, workspace string) (string, error) {
	raw, arm, err := e.client.call(ctx, "pane.list", map[string]any{"workspace_id": nullable(workspace)})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Panes []paneInfo `json:"panes"`
	}](raw, "pane_list", arm)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range res.Panes {
		focus := " "
		if p.Focused {
			focus = "*"
		}
		agent := p.Agent
		if agent == "" {
			agent = "-"
		}
		fmt.Fprintf(&b, "%s %s  %-10s  agent=%-12s %-9s %s\n", focus, p.PaneID, p.TabID, agent, p.AgentStatus, or(p.TerminalTitleStripped, or(p.TerminalTitle, "")))
	}
	b.WriteString(fmt.Sprintf("%d pane(s)\n", len(res.Panes)))
	return b.String(), nil
}

func (e *engine) paneSplit(ctx context.Context, paneID, direction, cwd string, focus bool) (string, error) {
	if direction == "" {
		direction = "right"
	}
	raw, arm, err := e.client.call(ctx, "pane.split", map[string]any{
		"target_pane_id": nullable(paneID), "direction": direction, "cwd": nullable(cwd), "focus": focus,
	})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Pane paneInfo `json:"pane"`
	}](raw, "pane_info", arm)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("pane %s (tab %s, workspace %s)\n", res.Pane.PaneID, res.Pane.TabID, res.Pane.WorkspaceID), nil
}

func (e *engine) paneSendText(ctx context.Context, paneID, text string) (string, error) {
	if _, _, err := e.client.call(ctx, "pane.send_text", map[string]any{"pane_id": paneID, "text": text}); err != nil {
		return "", err
	}
	return fmt.Sprintf("sent %d byte(s) to %s\n", len(text), paneID), nil
}

func (e *engine) paneSendKeys(ctx context.Context, paneID string, keys []string) (string, error) {
	if _, _, err := e.client.call(ctx, "pane.send_keys", map[string]any{"pane_id": paneID, "keys": keys}); err != nil {
		return "", err
	}
	return fmt.Sprintf("sent keys %s to %s\n", strings.Join(keys, " "), paneID), nil
}

func (e *engine) paneRun(ctx context.Context, paneID, command string) (string, error) {
	if _, _, err := e.client.call(ctx, "pane.send_text", map[string]any{"pane_id": paneID, "text": command}); err != nil {
		return "", err
	}
	// The herdr CLI's `pane run` atomically sends the text and Enter. Over the
	// socket that is send_text + a logical Enter key (validated end-to-end by
	// the check-herdr-pod bed's pane run + wait-output steps).
	if _, _, err := e.client.call(ctx, "pane.send_keys", map[string]any{"pane_id": paneID, "keys": []string{"enter"}}); err != nil {
		return "", err
	}
	return fmt.Sprintf("ran %q in %s\n", command, paneID), nil
}

func (e *engine) paneRead(ctx context.Context, paneID, source string, lines int) (string, error) {
	params := map[string]any{"pane_id": paneID, "source": wireSource(source, "recent"), "strip_ansi": true}
	if lines > 0 {
		params["lines"] = lines
	}
	raw, arm, err := e.client.call(ctx, "pane.read", params)
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Read paneReadResult `json:"read"`
	}](raw, "pane_read", arm)
	if err != nil {
		return "", err
	}
	return res.Read.Text, nil
}

func (e *engine) paneWaitOutput(ctx context.Context, paneID, match, regex, source string, timeoutMs int64) (string, error) {
	if match == "" && regex == "" {
		return "", fmt.Errorf("pane wait-output needs --match <text> or --regex <pattern>")
	}
	m := map[string]string{}
	if match != "" {
		m = map[string]string{"type": "substring", "value": match}
	} else {
		m = map[string]string{"type": "regex", "value": regex}
	}
	params := map[string]any{
		"pane_id": paneID, "source": wireSource(source, "recent"),
		"match": m, "timeout_ms": uint64(timeoutMs),
	}
	raw, arm, err := e.client.call(ctx, "pane.wait_for_output", params)
	if err != nil {
		return "", err
	}
	res, err := decodeResult[outputMatchedResult](raw, "output_matched", arm)
	if err != nil {
		return "", err
	}
	if res.MatchedLine != "" {
		return fmt.Sprintf("output matched in %s (revision %d): %s\n", res.PaneID, res.Revision, res.MatchedLine), nil
	}
	return fmt.Sprintf("output matched in %s (revision %d)\n", res.PaneID, res.Revision), nil
}

func (e *engine) agents(ctx context.Context) (string, error) {
	raw, arm, err := e.client.call(ctx, "agent.list", struct{}{})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Agents []agentInfo `json:"agents"`
	}](raw, "agent_list", arm)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, a := range res.Agents {
		focus := " "
		if a.Focused {
			focus = "*"
		}
		id := a.Name
		if id == "" {
			id = a.Agent
		}
		fmt.Fprintf(&b, "%s %-20s %-9s pane=%s workspace=%s %s\n", focus, or(id, "?"), a.AgentStatus, a.PaneID, a.WorkspaceID, a.Cwd)
	}
	b.WriteString(fmt.Sprintf("%d agent(s)\n", len(res.Agents)))
	return b.String(), nil
}

func (e *engine) agentGet(ctx context.Context, target string) (string, error) {
	raw, arm, err := e.client.call(ctx, "agent.get", map[string]any{"target": target})
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Agent agentInfo `json:"agent"`
	}](raw, "agent_info", arm)
	if err != nil {
		return "", err
	}
	a := res.Agent
	id := a.Name
	if id == "" {
		id = a.Agent
	}
	return fmt.Sprintf("%s  %s  pane=%s workspace=%s cwd=%s ready=%v\n",
		or(id, "?"), a.AgentStatus, a.PaneID, a.WorkspaceID, a.Cwd, a.InteractiveReady), nil
}

func (e *engine) agentWait(ctx context.Context, target, until string, timeoutMs int64) (string, error) {
	states := []string{}
	if until != "" {
		states = append(states, until)
	}
	params := map[string]any{"target": target}
	if len(states) > 0 {
		params["until"] = states
	}
	if timeoutMs > 0 {
		params["timeout_ms"] = uint64(timeoutMs)
	}
	if _, _, err := e.client.call(ctx, "agent.wait", params); err != nil {
		return "", err
	}
	return fmt.Sprintf("agent %s reached %s\n", target, or(until, "a settled state")), nil
}

func (e *engine) agentPrompt(ctx context.Context, target, text string, wait bool, timeoutMs int64) (string, error) {
	params := map[string]any{"target": target, "text": text}
	if wait {
		w := map[string]any{}
		if timeoutMs > 0 {
			w["timeout_ms"] = uint64(timeoutMs)
		}
		params["wait"] = w
	}
	raw, arm, err := e.client.call(ctx, "agent.prompt", params)
	if err != nil {
		return "", err
	}
	res, err := decodeResult[struct {
		Agent agentInfo `json:"agent"`
	}](raw, "agent_prompted", arm)
	if err != nil {
		return "", err
	}
	id := res.Agent.Name
	if id == "" {
		id = res.Agent.Agent
	}
	return fmt.Sprintf("prompted %s (%s)\n", or(id, target), res.Agent.AgentStatus), nil
}

func (e *engine) agentReport(ctx context.Context, paneID, source, agent, state, message string) (string, error) {
	params := map[string]any{
		"pane_id": paneID, "source": source, "agent": agent, "state": state,
	}
	if message != "" {
		params["message"] = message
	}
	if _, _, err := e.client.call(ctx, "pane.report_agent", params); err != nil {
		return "", err
	}
	return fmt.Sprintf("reported agent %s (%s) on %s: %s\n", agent, state, paneID, or(message, "-")), nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// parseTimeout resolves an authored check-step timeout into a duration.
func parseTimeout(raw any, def time.Duration) time.Duration {
	switch v := raw.(type) {
	case float64:
		return time.Duration(v) * time.Millisecond
	case string:
		if v == "" {
			return def
		}
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Duration(n) * time.Millisecond
		}
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
