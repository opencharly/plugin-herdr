# plugin-herdr

The `herdr` plugin for [opencharly/charly](https://github.com/opencharly/charly) —
a `charly herdr` CLI plus the declarative `herdr:` check verb for a
[Herdr](https://herdr.dev) terminal-multiplexer session. The plugin speaks the
herdr NDJSON socket API directly: no upstream `herdr` binary, no SDK.

## What it provides

| Capability | Surface |
|---|---|
| `command:herdr` | the `charly herdr` CLI — inspect and control a Herdr session |
| `verb:herdr` | the declarative `herdr:` check step any candy or box can bake into its plan |

## The command

`charly herdr status` pings the target and summarizes
workspaces/tabs/panes/agents. The command tree:

- `charly herdr status` / `charly herdr config` — ping, and show the resolved target.
- `charly herdr session snapshot` — the live session snapshot (JSON).
- `charly herdr workspace list|create`, `tab list|create`, `pane list|split|run|read|send-text|send-keys|wait-output`.
- `charly herdr agent list|get|wait|prompt|report`.

## Session targeting and the focused-session boundary

The focused Herdr session is OFF-LIMITS unless you are inside Herdr
(`HERDR_ENV=1` — you run in a herdr pane), you say `--focused` explicitly, or you
target another session explicitly:

| Target | Resolution |
|---|---|
| `--session <name>` | `~/.config/herdr/sessions/<name>/herdr.sock` |
| `--endpoint <host:port>` | TCP speaking the herdr NDJSON protocol (e.g. the pod-herdr socket bridge) |
| `HERDR_SOCKET_PATH` | the socket at that path |
| `HERDR_ENV=1` | the current (own) session socket |
| `--focused` | the focused session, explicitly |

`charly herdr` with no target outside herdr errors with guidance instead of
touching the focused session. Never `server.stop` an active session; use named
test sessions (`--session name`) for experiments.

## The verb

An authored `herdr: <method>` step resolves the in-venue herdr socket-bridge
port to a host-routable address over the reverse channel and probes with the SAME
NDJSON client the command uses. Methods: `ping`, `session-snapshot`,
`workspace-list`, `tab-list`, `pane-list`, `agent-list`, `pane-wait-output`,
`agent-wait`, `agent-prompt`. The verb skips under `charly check box` (no live
venue on a disposable `podman run --rm`).

```yaml
- check: the herdr pane prints the marker
  herdr:
      method: pane-wait-output
      pane: w1:p1
      match: herdr-marker
  context: [runtime]
```

## How to use it

Compose the plugin candy in a box or check bed's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-herdr/candy/plugin-herdr:<tag>'
```

## Example

```bash
charly herdr --session spike status
charly herdr --session spike workspace create --label demo
charly herdr --session spike pane split --direction right
charly herdr --session spike pane run w1:p2 "echo herdr-marker"
charly herdr --session spike pane wait-output w1:p2 --match herdr-marker
charly herdr --session spike agent list
```

## Layout

- `candy/plugin-herdr/` — the plugin module: `plugin.go`, `command.go`,
  `control.go`, `client.go`, `session.go`, `verb.go`, `params/cue_types_gen.go`,
  `schema/herdr.cue`, `cmd/serve/main.go`.
- `candy/plugin-herdr/charly.yml` — the `plugin-herdr:` candy entity.
- `charly.yml` — the root project manifest (`discover: candy`) plus the embedded
  `herdr-skill:` skill entity.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-automation:herdr` (projected from the embedded
  `herdr-skill:` entity).
- `/charly-automation:herdr-box` — the herdr stack box + `check-herdr-pod` bed.
- `/charly-internals:plugin` — the plugin/provider model.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
