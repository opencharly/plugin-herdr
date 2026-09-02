package herdr

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/opencharly/plugin-herdr/candy/plugin-herdr/params"
	"github.com/opencharly/spec/spec"
)

// TestLiveVerbReadOnly drives the REAL verb dispatch (runVerbHerdrResolved)
// against the live session — every read-only method. No fakes: the resolver
// hands the live session socket to the verb.
func TestLiveVerbReadOnly(t *testing.T) {
	if os.Getenv("HERDR_ENV") != "1" && os.Getenv("HERDR_SOCKET_PATH") == "" {
		t.Skip("no live herdr session; run inside herdr (HERDR_ENV=1) or set HERDR_SOCKET_PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tgt, err := resolveTarget(targetOpts{})
	if err != nil {
		t.Fatal(err)
	}
	resolve := func(context.Context, int) (string, error) { return tgt.Dial, nil }

	for _, tc := range []struct {
		name   string
		method string
	}{
		{"ping", "ping"},
		{"session-snapshot", "session-snapshot"},
		{"workspace-list", "workspace-list"},
		{"tab-list", "tab-list"},
		{"pane-list", "pane-list"},
		{"agent-list", "agent-list"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runVerbHerdrResolved(ctx, resolve, &spec.Op{}, params.HerdrInput{Method: tc.method})
			if err != nil {
				t.Fatalf("%s: %v", tc.method, err)
			}
			if strings.TrimSpace(out) == "" {
				t.Errorf("%s: empty output", tc.method)
			}
		})
	}
}

func TestVerbTimeoutParsing(t *testing.T) {
	if d := parseTimeout(nil, defaultVerbTimeout); d != defaultVerbTimeout {
		t.Errorf("parseTimeout(nil) = %v, want default", d)
	}
	if d := parseTimeout(float64(5000), defaultVerbTimeout); d != 5*time.Second {
		t.Errorf("parseTimeout(5000) = %v, want 5s", d)
	}
	if d := parseTimeout("2s", defaultVerbTimeout); d != 2*time.Second {
		t.Errorf("parseTimeout(2s) = %v, want 2s", d)
	}
}

func TestVerbWireSource(t *testing.T) {
	if s := wireSource("recent-unwrapped", "recent"); s != "recent_unwrapped" {
		t.Errorf("wireSource(recent-unwrapped) = %q", s)
	}
	if s := wireSource("", "recent"); s != "recent" {
		t.Errorf("wireSource(default) = %q", s)
	}
}
