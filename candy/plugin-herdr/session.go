package herdr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Session targeting mirrors the herdr agent skill's safety boundary as code:
// the FOCUSED session is off-limits unless the caller explicitly targets it
// (--focused), it is the caller's own session (HERDR_ENV=1 — the plugin runs
// inside a herdr pane), or it was reached through an explicit --session /
// --endpoint / HERDR_SOCKET_PATH target that is not the focused server.

// targetOpts is the resolved targeting the CLI carries (kong globals + env).
type targetOpts struct {
	Session  string // named session (~/.config/herdr/sessions/<name>/herdr.sock)
	Endpoint string // tcp host:port speaking the herdr NDJSON protocol
	Focused  bool   // explicit operator intent to target the focused session
}

// sessionTarget is a resolved, dialable target.
type sessionTarget struct {
	Dial    string // "unix:///path" or "tcp://host:port"
	Kind    string // "unix" | "tcp"
	Addr    string // socket path or host:port
	Source  string // how it was resolved, for `charly herdr config` output
	Focused bool   // true when this is the focused session
}

func (t sessionTarget) String() string { return t.Dial }

// herdrConfigDir reproduces herdr's config-dir resolution (HERDR_CONFIG_DIR,
// then XDG_CONFIG_HOME/herdr, then ~/.config/herdr).
func herdrConfigDir() string {
	if d := os.Getenv("HERDR_CONFIG_DIR"); d != "" {
		return d
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "herdr")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "herdr")
	}
	return filepath.Join(home, ".config", "herdr")
}

func focusedSocketPath() string {
	if p := os.Getenv("HERDR_SOCKET_PATH"); p != "" {
		return p
	}
	return filepath.Join(herdrConfigDir(), "herdr.sock")
}

func namedSessionSocketPath(name string) string {
	return filepath.Join(herdrConfigDir(), "sessions", name, "herdr.sock")
}

// resolveTarget applies the targeting ladder documented above.
func resolveTarget(o targetOpts) (sessionTarget, error) {
	if o.Session != "" {
		return sessionTarget{
			Dial: "unix://" + namedSessionSocketPath(o.Session),
			Kind: "unix", Addr: namedSessionSocketPath(o.Session),
			Source: "session",
		}, nil
	}
	if o.Endpoint != "" {
		// Accept both "host:port" and "tcp://host:port" spellings (the bed and
		// docs use the prefixed form; a bare host:port is the wire convention).
		ep := strings.TrimPrefix(o.Endpoint, "tcp://")
		return sessionTarget{Dial: "tcp://" + ep, Kind: "tcp", Addr: ep, Source: "endpoint"}, nil
	}
	if p := os.Getenv("HERDR_SOCKET_PATH"); p != "" {
		// An explicit socket path is a deliberate external target (a named
		// session socket someone exported); treat it as operator intent.
		return sessionTarget{Dial: "unix://" + p, Kind: "unix", Addr: p, Source: "HERDR_SOCKET_PATH", Focused: p == focusedSocketPath()}, nil
	}
	if os.Getenv("HERDR_ENV") == "1" {
		// We are INSIDE a herdr pane: the current session is ours to inspect
		// and control (the agent-skill contract).
		p := focusedSocketPath()
		return sessionTarget{Dial: "unix://" + p, Kind: "unix", Addr: p, Source: "herdr-env", Focused: true}, nil
	}
	if o.Focused {
		p := focusedSocketPath()
		return sessionTarget{Dial: "unix://" + p, Kind: "unix", Addr: p, Source: "focused", Focused: true}, nil
	}
	return sessionTarget{}, fmt.Errorf(
		"no Herdr session targeted: the focused session is off-limits from outside. " +
			"Use --session <name> (named test session), --endpoint <host:port> (venue bridge), " +
			"--focused (explicit operator intent), or run from inside Herdr (HERDR_ENV=1)")
}
