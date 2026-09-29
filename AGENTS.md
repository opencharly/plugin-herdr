# AGENTS.md — plugin-herdr

Standalone plugin repo for the Herdr terminal-multiplexer surface
(`command:herdr` + `verb:herdr`). The plugin is a Go module at
`candy/plugin-herdr/` (module path
`github.com/opencharly/plugin-herdr/candy/plugin-herdr`); the root `charly.yml`
declares `discover: candy` so the repo is a project and its candy is scanned,
and carries the embedded `herdr-skill:` skill entity.

Canonical files:

- `candy/plugin-herdr/charly.yml` — the `plugin-herdr:` candy entity (`plugin:`
  block, `plan:` check).
- `candy/plugin-herdr/` — the Go source: `plugin.go`, `command.go`,
  `control.go`, `client.go`, `session.go`, `verb.go`, `params/cue_types_gen.go`,
  `schema/herdr.cue`, `cmd/serve/main.go`.
- `charly.yml` — the root manifest (`discover: candy`) + the `herdr-skill:`
  skill entity.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-automation:herdr` — the `charly herdr` command + `herdr:` verb
  reference (projected from the embedded `herdr-skill:` entity). Load before
  changing the command tree or verb.
- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the dual-class `command` + `verb` provider model, the per-plugin
  CUE-schema contract. Load before touching the provider or schema.
- `/charly-check:check` — the declarative check-step surface the `herdr:` verb
  is authored through.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-herdr/` — compile the plugin module.
- `go test ./...` in `candy/plugin-herdr/` — the plugin's fake-socket wire tests
  (`go test -run TestLiveReadOnly -v .` is the read-only live smoke test).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The R10 witness is the `check-herdr-pod` bed in the `pod-herdr` repo.

## Modify this repo

- Edit the `plugin-herdr:` candy entity, the Go source, and `schema/herdr.cue`
  **together** — the schema is the single source for the verb's `params/`
  struct.
- Keep the focused-session safety boundary intact: never silently inspect or
  mutate the focused session; keep the `herdr-skill:` entity in step with any
  command-tree change.

## Landing

- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Load
  `/charly-internals:git-workflow` before any git/PR action; history lives in
  `CHANGELOG/`. Do not restate its rules here.
