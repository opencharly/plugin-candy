---
name: candy
description: '`charly candy set` / `charly candy add-<fmt>` — mutate a candy''s charly.yml
  safely

  (set a dot-path value, or append packages to a distro-format section). Use instead

  of hand-editing candy manifests. `charly candy params <name>` regenerates a plugin

  candy''s Go params from its own `schema/*.cue` — the ONLY supported way to do that

  from a shell, and it needs nothing but the charly binary.'
---

# charly candy — safe candy-manifest mutation

`charly candy` mutates a candy's `charly.yml` safely, descending into the entity's
`candy:` body. Served by `candy/plugin-candy` (`command:candy`).

## Command tree

- `charly candy set <name> <path> <value>` — set a dot-path value on
  `candy/<name>/charly.yml` (e.g. a tunable or a nested field).
- `charly candy add-<fmt> <name> <pkg…>` — append packages to the distro-format
  section `<fmt>` (e.g. `add-pac`, `add-deb`, `add-rpm`). The verb set derives from
  `sectionDistroPath`, so it cannot drift from the mapped formats.
- `charly candy params <name> [--check]` — regenerate
  `candy/<name>/params/cue_types_gen.go` from `candy/<name>/schema/*.cue`. This is
  the pipeline's ONLY documented entry point: concatenate the schema (the
  `schemaconcat` contract), head it `package params` + `@go(params)`, run the pinned
  `cue exp gengotypes` (the `cuetoolchain` pin — provisioned checksum-verified into
  the charly cache, so no `cue` install and no checkout are needed), then double
  every json tag with a yaml tag (the `schemaretag` contract). `--check` writes
  nothing: it exits non-zero when the committed file differs from that pipeline
  OUTPUT, which is the drift gate a plugin repo's CI should run. `--schema/--pkg/--out`
  is the explicit form for a schema that is not a candy's.

## When to use it

To change a candy manifest programmatically instead of hand-editing YAML — the
mutation respects the compact node form (`<name>: {candy: {...}}`) and the
schema, so it cannot produce a malformed manifest.

## Cross-References

- `/charly-image:layer` — candy authoring (plan steps, services, packages).
- `/charly-image:image` — box composition and charly.yml shapes.
