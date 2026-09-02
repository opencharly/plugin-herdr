package herdr

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencharly/plugin-herdr/candy/plugin-herdr/params"
	"github.com/opencharly/spec/spec"
)

func TestVerbDispatch(t *testing.T) {
	f := startFakeHerdr(t, "tcp")
	defer f.close()
	addr := strings.TrimPrefix(f.url, "tcp://")
	resolve := func(context.Context, int) (string, error) { return addr, nil }

	cases := []struct {
		name    string
		in      params.HerdrInput
		wantHas string
	}{
		{"ping", params.HerdrInput{Method: "ping"}, "pong"},
		{"session-snapshot", params.HerdrInput{Method: "session-snapshot"}, "workspaces: 1"},
		{"workspace-list", params.HerdrInput{Method: "workspace-list"}, "w1"},
		{"tab-list", params.HerdrInput{Method: "tab-list"}, "w1:t1"},
		{"pane-list", params.HerdrInput{Method: "pane-list"}, "w1:p1"},
		{"agent-list", params.HerdrInput{Method: "agent-list"}, "pi"},
		{"pane-wait-output", params.HerdrInput{Method: "pane-wait-output", Pane: "w1:p1", Match: "test result"}, "output matched"},
		{"agent-wait", params.HerdrInput{Method: "agent-wait", Agent: "pi", Until: "idle"}, "reached"},
		{"agent-prompt", params.HerdrInput{Method: "agent-prompt", Agent: "pi", Text: "hi"}, "prompted"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			op := &spec.Op{}
			out, err := runVerbHerdrResolved(ctx, resolve, op, tc.in)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if !strings.Contains(out, tc.wantHas) {
				t.Errorf("%s: out = %q, want contains %q", tc.name, out, tc.wantHas)
			}
		})
	}
}

func TestVerbUnknownMethod(t *testing.T) {
	f := startFakeHerdr(t, "tcp")
	defer f.close()
	addr := strings.TrimPrefix(f.url, "tcp://")
	resolve := func(context.Context, int) (string, error) { return addr, nil }
	ctx := context.Background()
	_, err := runVerbHerdrResolved(ctx, resolve, &spec.Op{}, params.HerdrInput{Method: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown herdr method") {
		t.Fatalf("err = %v, want unknown-method error", err)
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
	if s := wireSource("recent-unwrapped", "recent"); s != "recent_unwrapped" {
		t.Errorf("wireSource(recent-unwrapped) = %q", s)
	}
}
