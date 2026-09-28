# plugin-candy

The `charly candy` authoring surface for OpenCharly — served as a charly
`command:candy` plugin (compiled-in).

`charly candy` mutates a candy's `charly.yml` safely, descending into the
entity's `candy:` body — a dot-path `set`, or an append to a distro-format
package section. It respects the compact node form and the schema, so it cannot
produce a malformed manifest.

## What it provides

| Capability | Surface |
|---|---|
| `command:candy` | the `charly candy` CLI — `set` + `add-<fmt>` (`add-rpm`, `add-deb`, `add-pac`, `add-aur`, `add-apk`) |

## The command

```
charly candy set <name> <path> <value>   # set a dot-path value on candy/<name>/charly.yml
charly candy add-<fmt> <name> <pkg…>     # append packages to the distro-format section <fmt>
```

This is the TOP-LEVEL `charly candy` authoring tree (edit an EXISTING candy's
fields). It is NOT `charly box new candy` (scaffold a new candy dir), which is a
different command served by `candy/plugin-box` (`command:new`).

## How to use it

Compose the plugin candy in a box's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-candy/candy/plugin-candy:<tag>'
```

## Layout

- `candy/plugin-candy/` — the plugin module: `plugin.go`, `provider.go`,
  `command.go`, `schema/candy.cue`, `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy`).
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-candy-cli:candy` (projected from the embedded
  `candy-skill:` entity).
- `/charly-image:layer` — candy authoring (plan steps, services, packages).
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
