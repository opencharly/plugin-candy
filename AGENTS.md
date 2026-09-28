# AGENTS.md — plugin-candy

Standalone plugin repo owning the `charly candy` authoring surface
(`command:candy`, compiled-in). The plugin is a Go module at
`candy/plugin-candy/` (module path
`github.com/opencharly/plugin-candy/candy/plugin-candy`); the root `charly.yml`
only declares `discover: candy` so the repo is a project and its candy is
scanned.

Canonical files:

- `candy/plugin-candy/charly.yml` — the `plugin-candy:` candy entity and the
  embedded `candy-skill:` skill entity.
- `candy/plugin-candy/plugin.go` — `NewProvider()`/`NewMeta()`/`CliMain` (the
  dual-mode command shape).
- `candy/plugin-candy/provider.go` — the `Invoke(OpRun)` surface.
- `candy/plugin-candy/command.go` — the comment-preserving `yaml.Node` mutation
  (`candySet` / `appendCandyPackages`).
- `candy/plugin-candy/schema/candy.cue` — the self-contained plugin schema.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-candy-cli:candy` — the `charly candy` command reference (projected
  from the embedded `candy-skill:` entity). Load before changing the command
  tree or the mutation logic.
- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the `command` provider class, the per-plugin CUE-schema contract.
- `/charly-image:layer` — the candy authoring surface the command mutates.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-candy/` — compile the plugin module.
- `go test ./...` in `candy/plugin-candy/` — the plugin's Go tests (the command
  mutation + schema-serve seams).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The R10 witness is the disposable `check-commands-local` bed (in
  `opencharly/charly`): `charly candy --help` exits 0.

## Modify this repo

- Edit the `plugin-candy:` candy entity, the Go source, and `schema/candy.cue`
  **together** — the schema is the single source for the plugin's served
  declaration surface.
- Keep the `candy-skill:` entity in step with any command-tree change — it is the
  projected source for `/charly-candy-cli:candy`.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
