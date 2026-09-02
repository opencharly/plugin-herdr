# plugin-herdr

The “herdr” plugin candy for [opencharly/charly](https://github.com/opencharly/charly): a
`command:herdr` CLI plus the declarative `herdr:` check verb, speaking the
Herdr NDJSON socket API directly (no upstream `herdr` binary needed).

- `charly herdr` — inspect and control a Herdr session: `status`, `session snapshot`,
  `workspace list|create`, `tab list|create`, `pane list|split|run|read|send-text|send-keys|wait-output`,
  `agent list|get|wait|prompt|report`, `config`.
- `herdr:` — the declarative check verb for beds: `ping`, `session-snapshot`,
  `workspace-list`, `tab-list`, `pane-list`, `agent-list`, `pane-wait-output`,
  `agent-wait`, `agent-prompt`.

The plugin is an OUT-OF-TREE external plugin: projects compose it via the
`@github.com/opencharly/plugin-herdr/candy/plugin-herdr:<ref>` candy ref and charly
connects it OUT-OF-PROCESS by word at runtime (the `herdr:` verb + `charly herdr`
CLI both dispatch through the `cmd/serve` gRPC shim) — zero charly-module import,
per the kernel/plugin boundary law. The same Go core serves both placements (R3).

## Session targeting and the focused-session boundary

The focused Herdr session is OFF-LIMITS unless you are inside Herdr (`HERDR_ENV=1` — you run in
a herdr pane), you say `--focused` explicitly, or you target another session explicitly:

| Target | Resolution |
|---|---|
| `--session <name>` | `~/.config/herdr/sessions/<name>/herdr.sock` |
| `--endpoint <host:port>` | TCP speaking the herdr NDJSON protocol (e.g. the pod-herdr socket bridge) |
| `HERDR_SOCKET_PATH` | the socket at that path |
| `HERDR_ENV=1` | the current (own) session socket |
| `--focused` | the focused session, explicitly |

`charly herdr` with no target outside herdr errors with guidance instead of touching the
focused session — the herdr agent-skill safety rule as code. Never `server.stop` an active
session; use named test sessions (`--session name`) for experiments.

## Example

```bash
charly herdr --session spike status
charly herdr --session spike workspace create --label demo
charly herdr --session spike pane split --direction right
charly herdr --session spike pane run w1:p2 "echo herdr-marker"
charly herdr --session spike pane wait-output w1:p2 --match herdr-marker
charly herdr --session spike agent list
```

## Building / testing

```bash
cd candy/plugin-herdr
go build ./... && go test ./...
# live smoke test against your own herdr session (read-only):
go test -run TestLiveReadOnly -v .
```

The wire contract is documented by the herdr API schema (`herdr api schema --json`, protocol
20) and pinned by the fake-socket tests (`client_test.go`).

The Go module lives at `candy/plugin-herdr/` with module path
`github.com/opencharly/plugin-herdr/candy/plugin-herdr`; the charly resolver fetches this repo
at the pinned tag and the compiled-in wiring imports the module at that path.
