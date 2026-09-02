package herdr

import (
	"strings"
	"testing"
)

func TestResolveTargetEndpointSpellings(t *testing.T) {
	for _, ep := range []string{"127.0.0.1:8095", "tcp://127.0.0.1:8095"} {
		tgt, err := resolveTarget(targetOpts{Endpoint: ep})
		if err != nil {
			t.Fatal(err)
		}
		if tgt.Dial != "tcp://127.0.0.1:8095" || tgt.Addr != "127.0.0.1:8095" {
			t.Errorf("endpoint %q -> %+v", ep, tgt)
		}
	}
}

func TestResolveTargetFocusedRefused(t *testing.T) {
	t.Setenv("HERDR_SOCKET_PATH", "")
	t.Setenv("HERDR_ENV", "0")
	if _, err := resolveTarget(targetOpts{}); err == nil || !strings.Contains(err.Error(), "off-limits") {
		t.Fatalf("err = %v, want focused-session guard", err)
	}
}
