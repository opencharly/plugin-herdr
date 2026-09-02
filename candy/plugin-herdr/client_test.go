package herdr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// The protocol truth is the REAL herdr server — these tests run against the
// live session (HERDR_ENV=1 — the current session is ours to inspect — or
// HERDR_SOCKET_PATH). Nothing here creates, splits, moves, or closes anything:
// the mutating surfaces are exercised LIVE by the check-herdr-pod R10 bed
// (charly check run check-herdr-pod) on a disposable pod. There is no fake
// server: charly tests things on live systems.

func hasLiveHerdr() bool {
	return os.Getenv("HERDR_ENV") == "1" || os.Getenv("HERDR_SOCKET_PATH") != ""
}

func liveEngine(t *testing.T) *engine {
	t.Helper()
	tgt, err := resolveTarget(targetOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return &engine{client: newNDJSONClient(tgt.Dial), target: tgt}
}

func TestLiveReadOnly(t *testing.T) {
	if !hasLiveHerdr() {
		t.Skip("no live herdr session; run inside herdr (HERDR_ENV=1) or set HERDR_SOCKET_PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	eng := liveEngine(t)
	for _, tc := range []struct {
		name string
		fn   func(context.Context) (string, error)
	}{
		{"status", eng.status},
		{"workspaces", eng.workspaces},
		{"tabs", func(c context.Context) (string, error) { return eng.tabs(c, "") }},
		{"panes", func(c context.Context) (string, error) { return eng.panes(c, "") }},
		{"agents", eng.agents},
	} {
		out, err := tc.fn(ctx)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		t.Logf("%s:\n%s", tc.name, out)
	}
}

// TestLiveEmptyPaneWaitOutput is the B12 live test for the focused-pane
// default in pane-wait-output: resolve the REAL focused pane via pane.current,
// read a real fragment of its output, then wait for it with an EMPTY pane id.
// Without the pane.current resolution the real server rejects the request
// (recv: EOF) — the test fails.
func TestLiveEmptyPaneWaitOutput(t *testing.T) {
	if !hasLiveHerdr() {
		t.Skip("no live herdr session")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	eng := liveEngine(t)

	raw, arm, err := eng.client.call(ctx, "pane.current", struct{}{})
	if err != nil {
		t.Fatalf("pane.current: %v", err)
	}
	cur, err := decodeResult[struct {
		Pane paneInfo `json:"pane"`
	}](raw, "pane_current", arm)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Pane.PaneID == "" {
		t.Fatal("no focused pane in the live session")
	}

	text, err := eng.paneRead(ctx, cur.Pane.PaneID, "recent-unwrapped", 60)
	if err != nil {
		t.Fatal(err)
	}
	frag := firstNonEmptyLine(text)
	if frag == "" {
		t.Fatal("no readable output on the focused pane")
	}
	if len(frag) > 60 {
		frag = frag[:60]
	}

	out, err := eng.paneWaitOutput(ctx, "", frag, "", "recent", 15000)
	if err != nil {
		t.Fatalf("empty-pane wait: %v", err)
	}
	if !strings.Contains(out, "output matched") {
		t.Errorf("out = %q, want an output-matched verdict", out)
	}
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
