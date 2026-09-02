package herdr

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHerdr is a minimal herdr NDJSON server that answers the methods the
// plugin's tests exercise. It pins the wire contract: request
// {"id","method","params"} → response {"id", "result": {arm}}, matched by id.
type fakeHerdr struct {
	ln     net.Listener
	url    string
	mu     sync.Mutex
	calls  []string
	params []string
}

func (f *fakeHerdr) record(method string, params json.RawMessage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method)
	f.params = append(f.params, string(params))
}

func startFakeHerdr(t *testing.T, network string) *fakeHerdr {
	t.Helper()
	var ln net.Listener
	var err error
	if network == "unix" {
		dir := t.TempDir()
		sock := filepath.Join(dir, "herdr.sock")
		ln, err = net.Listen("unix", sock)
		if err != nil {
			t.Fatal(err)
		}
		f := &fakeHerdr{ln: ln, url: "unix://" + sock}
		go f.serve()
		return f
	}
	ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeHerdr{ln: ln, url: "tcp://" + ln.Addr().String()}
	go f.serve()
	return f
}

func (f *fakeHerdr) close() { f.ln.Close() }


func (f *fakeHerdr) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeHerdr) handle(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req struct {
			ID     string          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := dec.Decode(&req); err != nil {
			return
		}
		f.record(req.Method, req.Params)
		result := f.respond(req.Method, req.Params)
		if result == nil {
			result = map[string]any{"type": "ok"}
		}
		_ = enc.Encode(map[string]any{"id": req.ID, "result": result})
	}
}

func (f *fakeHerdr) respond(method string, params json.RawMessage) map[string]any {
	switch method {
	case "ping":
		return map[string]any{"type": "pong", "version": "0.8.2", "protocol": 20}
	case "session.snapshot":
		return map[string]any{
			"type": "session_snapshot",
			"snapshot": map[string]any{
				"version": "0.8.2", "protocol": 20,
				"workspaces": []any{
					map[string]any{"workspace_id": "w1", "number": 1, "label": "lab", "focused": true, "pane_count": 2, "tab_count": 1, "active_tab_id": "w1:t1", "agent_status": "idle"},
				},
				"tabs":                 []any{map[string]any{"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "1", "focused": true, "pane_count": 2, "agent_status": "idle"}},
				"panes":                []any{map[string]any{"pane_id": "w1:p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "w1:t1", "focused": true, "agent_status": "working", "agent": "pi"}},
				"agents":               []any{map[string]any{"agent": "pi", "agent_status": "working", "cwd": "/work", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": true}},
				"focused_workspace_id": "w1", "focused_tab_id": "w1:t1", "focused_pane_id": "w1:p1",
			},
		}
	case "workspace.list":
		return map[string]any{
			"type": "workspace_list",
			"workspaces": []any{
				map[string]any{"workspace_id": "w1", "number": 1, "label": "lab", "focused": true, "pane_count": 1, "tab_count": 1, "active_tab_id": "w1:t1", "agent_status": "idle"},
			},
		}
	case "workspace.create":
		return map[string]any{
			"type":      "workspace_created",
			"workspace": map[string]any{"workspace_id": "w2", "number": 2, "label": "new", "focused": false, "pane_count": 1, "tab_count": 1, "active_tab_id": "w2:t1", "agent_status": "idle"},
			"tab":       map[string]any{"tab_id": "w2:t1", "workspace_id": "w2", "number": 1, "label": "1", "focused": true, "pane_count": 1, "agent_status": "idle"},
			"root_pane": map[string]any{"pane_id": "w2:p1", "terminal_id": "term2", "workspace_id": "w2", "tab_id": "w2:t1", "focused": true, "agent_status": "idle"},
		}
	case "tab.list":
		return map[string]any{
			"type": "tab_list",
			"tabs": []any{map[string]any{"tab_id": "w1:t1", "workspace_id": "w1", "number": 1, "label": "1", "focused": true, "pane_count": 1, "agent_status": "idle"}},
		}
	case "pane.current":
		return map[string]any{
			"type": "pane_current",
			"pane": map[string]any{"pane_id": "w1:p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "w1:t1", "focused": true, "agent_status": "unknown"},
		}
	case "pane.list":
		return map[string]any{
			"type":  "pane_list",
			"panes": []any{map[string]any{"pane_id": "w1:p1", "terminal_id": "term1", "workspace_id": "w1", "tab_id": "w1:t1", "focused": true, "agent_status": "working", "agent": "pi", "cwd": "/work"}},
		}
	case "pane.split":
		return map[string]any{
			"type": "pane_info",
			"pane": map[string]any{"pane_id": "w1:p2", "terminal_id": "term3", "workspace_id": "w1", "tab_id": "w1:t1", "focused": false, "agent_status": "unknown"},
		}
	case "pane.read":
		return map[string]any{
			"type": "pane_read",
			"read": map[string]any{"pane_id": "w1:p1", "workspace_id": "w1", "tab_id": "w1:t1", "source": "recent", "format": "text", "text": "hello from the pane", "revision": 5, "truncated": false},
		}
	case "pane.wait_for_output":
		return map[string]any{
			"type": "output_matched", "pane_id": "w1:p1", "revision": 7, "matched_line": "test result: ok",
			"read": map[string]any{"pane_id": "w1:p1", "workspace_id": "w1", "tab_id": "w1:t1", "source": "recent", "format": "text", "text": "test result: ok", "revision": 7, "truncated": false},
		}
	case "agent.list":
		return map[string]any{
			"type":   "agent_list",
			"agents": []any{map[string]any{"agent": "pi", "name": "pi", "agent_status": "working", "cwd": "/work", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": true}},
		}
	case "agent.get":
		return map[string]any{
			"type":  "agent_info",
			"agent": map[string]any{"agent": "pi", "name": "pi", "agent_status": "working", "cwd": "/work", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": true},
		}
	case "agent.wait":
		return map[string]any{"type": "wait_matched", "event": map[string]any{"event": "agent_status", "data": map[string]any{}}}
	case "agent.prompt":
		return map[string]any{
			"type":  "agent_prompted",
			"agent": map[string]any{"agent": "pi", "name": "pi", "agent_status": "working", "cwd": "/work", "workspace_id": "w1", "tab_id": "w1:t1", "pane_id": "w1:p1", "focused": true},
		}
	case "pane.send_text", "pane.send_keys", "pane.report_agent":
		return map[string]any{"type": "ok"}
	}
	return nil
}

func TestClientPing(t *testing.T) {
	f := startFakeHerdr(t, "unix")
	defer f.close()
	c := newNDJSONClient(f.url)
	raw, arm, err := c.call(context.Background(), "ping", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	pong, err := decodeResult[pongResult](raw, "pong", arm)
	if err != nil {
		t.Fatal(err)
	}
	if pong.Version != "0.8.2" || pong.Protocol != 20 {
		t.Errorf("pong = %+v", pong)
	}
}

func TestClientWorkspaceCreate(t *testing.T) {
	f := startFakeHerdr(t, "unix")
	defer f.close()
	eng, err := newTestEngine(f.url)
	if err != nil {
		t.Fatal(err)
	}
	out, err := eng.workspaceCreate(context.Background(), "new", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "w2") || !strings.Contains(out, "w2:p1") {
		t.Errorf("workspaceCreate out = %q", out)
	}
}

func TestClientErrorEnvelope(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "err.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		dec := json.NewDecoder(conn)
		enc := json.NewEncoder(conn)
		var req map[string]any
		_ = dec.Decode(&req)
		_ = enc.Encode(map[string]any{"id": req["id"], "error": map[string]string{"code": "not_found", "message": "no such pane"}})
	}()
	c := newNDJSONClient("unix://" + sock)
	_, _, err = c.call(context.Background(), "pane.read", map[string]string{"pane_id": "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no such pane") {
		t.Errorf("err = %v", err)
	}
}

func TestClientArmTypeTolerant(t *testing.T) {
	// A server may emit the discriminator at the envelope level (top-level
	// "type") instead of inside result — the client must accept both.
	dir := t.TempDir()
	sock := filepath.Join(dir, "mix.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		dec := json.NewDecoder(conn)
		enc := json.NewEncoder(conn)
		var req map[string]any
		_ = dec.Decode(&req)
		_ = enc.Encode(map[string]any{"id": req["id"], "type": "pong", "result": map[string]any{"version": "0.8.2", "protocol": 20}})
	}()
	c := newNDJSONClient("unix://" + sock)
	raw, arm, err := c.call(context.Background(), "ping", struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	pong, err := decodeResult[pongResult](raw, "pong", arm)
	if err != nil {
		t.Fatal(err)
	}
	if pong.Version != "0.8.2" {
		t.Errorf("pong = %+v", pong)
	}
}

// TestPaneWaitOutputFocusedResolution pins the focused-pane default (B12): an
// empty paneID must resolve pane.current FIRST and then wait with the resolved id.
func TestPaneWaitOutputFocusedResolution(t *testing.T) {
	f := startFakeHerdr(t, "unix")
	defer f.close()
	eng, err := newTestEngine(f.url)
	if err != nil {
		t.Fatal(err)
	}
	out, err := eng.paneWaitOutput(context.Background(), "", "test result", "", "recent", 5000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "output matched") {
		t.Errorf("out = %q", out)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) < 2 || f.calls[0] != "pane.current" || f.calls[1] != "pane.wait_for_output" {
		t.Fatalf("calls = %v, want pane.current then pane.wait_for_output", f.calls)
	}
	if len(f.params) < 2 || !strings.Contains(f.params[1], "\"pane_id\":\"w1:p1\"") {
		t.Fatalf("wait params = %v, want the resolved pane_id on the wire", f.params)
	}
}

// TestPaneWaitOutputNoFocusedPane: a venue with no focused pane must error clearly.
func TestPaneWaitOutputNoFocusedPane(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "nofocus.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		dec := json.NewDecoder(conn)
		enc := json.NewEncoder(conn)
		for {
			var req struct {
				ID     string          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := dec.Decode(&req); err != nil {
				return
			}
			if req.Method == "pane.current" {
				_ = enc.Encode(map[string]any{"id": req.ID, "result": map[string]any{"type": "pane_current", "pane": map[string]any{}}})
			} else {
				_ = enc.Encode(map[string]any{"id": req.ID, "result": map[string]any{"type": "ok"}})
			}
		}
	}()
	eng := &engine{client: newNDJSONClient("unix://" + sock), target: sessionTarget{Dial: "unix://" + sock}}
	_, err = eng.paneWaitOutput(context.Background(), "", "marker", "", "recent", 5000)
	if err == nil || !strings.Contains(err.Error(), "no focused pane") {
		t.Fatalf("err = %v, want no-focused-pane error", err)
	}
}

// TestLiveReadOnly exercises the plugin's engine against the REAL local herdr
// (read-only methods only) when the test runs inside a herdr session (HERDR_ENV)
// or with HERDR_SOCKET_PATH set. It is the live smoke test for development;
// CI (no herdr) skips it.
func TestLiveReadOnly(t *testing.T) {
	if os.Getenv("HERDR_ENV") != "1" && os.Getenv("HERDR_SOCKET_PATH") == "" {
		t.Skip("no live herdr session; set HERDR_SOCKET_PATH or run inside herdr (HERDR_ENV=1)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	target, err := resolveTarget(targetOpts{})
	if err != nil {
		t.Fatal(err)
	}
	eng := &engine{client: newNDJSONClient(target.Dial), target: target}
	if out, err := eng.status(ctx); err != nil {
		t.Fatalf("status: %v", err)
	} else {
		t.Logf("status:\n%s", out)
	}
	if out, err := eng.workspaces(ctx); err != nil {
		t.Fatalf("workspaces: %v", err)
	} else {
		t.Logf("workspaces:\n%s", out)
	}
	if out, err := eng.agents(ctx); err != nil {
		t.Fatalf("agents: %v", err)
	} else {
		t.Logf("agents:\n%s", out)
	}
}

func newTestEngine(url string) (*engine, error) {
	return &engine{client: newNDJSONClient(url), target: sessionTarget{Dial: url}}, nil
}
