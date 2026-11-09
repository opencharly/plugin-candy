package candy

// params.go — `charly candy params <candy>`: regenerate a plugin candy's Go params from its own
// `candy/<name>/schema/*.cue`, using NOTHING but the charly binary.
//
// WHY THIS EXISTS (R4a, opencharly/charly#829): a plugin candy's `schema/*.cue` is a PUBLISHED
// surface — `charly docs generate` renders it into `reference/plugin/<plugin>/plugin-*.md` — and a
// published page may only name commands that work with the `charly` binary alone. Before this verb
// the only way to regenerate was a checkout-relative recipe (`cd` into a spec checkout, run
// `internal/schemagen -mode=concat`, run `cue exp gengotypes`, run `-mode=retag`), so the pages had
// to publish either a forbidden path or nothing. The verb generates in-process instead: the schema
// is concatenated, composed as one CUE module instance offline, and projected to Go through the CUE
// Go API — no `cue` CLI, no checkout, and no toolchain provisioning on the generation leg. The
// pinned `cue` CLI remains the authoring tool for editing schema sources.
//
// THE PIPELINE IS NOT HERE. It is `spec/schemaparams.Generate` — ONE public entry point that
// sequences the step contracts (`schemaconcat` for the concatenation, `schemacompose` for the one
// offline compose, the in-process CUE-API generator, `schemaretag` for the tag normalization) behind
// one call. This verb is a THIN WRAPPER over it: it derives the paths, calls the pipeline, and does
// the write-or-compare.
//
// That is the R3 point. Before `schemaparams` this file carried its own copy of concat → header →
// `cue exp gengotypes` → retag, and spec's own generator carried a second copy of the same sequence
// — two implementations of one behaviour that could drift. The copy is DELETED here, not wrapped:
// a wrapper that kept its own sequence would have been a THIRD copy wearing the name of a fix.
//
// --check compares the COMMITTED file against the pipeline's OUTPUT (never a grep, and never a hash
// of a previous run): that is the only comparison that can catch a generated file gone stale, which
// is why a plugin repo's CI can run `charly candy params <name> --check` as its drift gate.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/schemaparams"
)

const paramsUsage = `usage: charly candy params <name> [--check]
       charly candy params --schema <dir> --pkg <params> --out <file> [--check]

Regenerate a plugin candy's Go params from its own schema/*.cue — nothing but the
charly binary is needed: the schema is composed and generated in-process through the
CUE Go API, so there is no cue install, no checkout, and no toolchain provisioning.
The pinned cue CLI remains the authoring tool for schema sources.

  <name>            the candy whose candy/<name>/schema/*.cue is the source and whose
                    candy/<name>/params/cue_types_gen.go is regenerated
  --check           do not write: exit non-zero when the committed file differs from the
                    pipeline output (the drift gate a plugin repo's CI runs)
  --schema/--pkg/--out
                    explicit form, for a schema that is not a candy's (spec's own
                    schema/ + package spec, say)`

type paramsOpts struct {
	name   string
	schema string
	pkg    string
	out    string
	check  bool
}

// runCandyParams parses the two accepted forms and runs the pipeline.
func runCandyParams(args []string) error {
	var o paramsOpts
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "-h", "--help", "help":
			fmt.Println(paramsUsage)
			return nil
		case "--check":
			o.check = true
		case "--schema", "--pkg", "--out":
			if i+1 >= len(args) {
				return fmt.Errorf("%s needs a value\n%s", a, paramsUsage)
			}
			i++
			switch a {
			case "--schema":
				o.schema = args[i]
			case "--pkg":
				o.pkg = args[i]
			case "--out":
				o.out = args[i]
			}
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unknown flag %q\n%s", a, paramsUsage)
			}
			if o.name != "" {
				return fmt.Errorf("unexpected extra argument %q\n%s", a, paramsUsage)
			}
			o.name = a
		}
	}

	if o.name == "" && o.schema == "" {
		return fmt.Errorf("%s", paramsUsage)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// The candy form derives all three paths from the candy's own directory.
	if o.name != "" {
		if o.schema == "" {
			o.schema = filepath.Join(cwd, kit.DefaultCandyDir, o.name, "schema")
		}
		if o.pkg == "" {
			o.pkg = "params"
		}
		if o.out == "" {
			o.out = filepath.Join(cwd, kit.DefaultCandyDir, o.name, "params", "cue_types_gen.go")
		}
	}
	if o.pkg == "" {
		o.pkg = "params"
	}
	if o.out == "" {
		return fmt.Errorf("--out is required when --schema is given\n%s", paramsUsage)
	}
	if fi, err := os.Stat(o.schema); err != nil || !fi.IsDir() {
		return fmt.Errorf("no schema directory at %s (a plugin candy keeps its CUE schema in candy/<name>/schema/*.cue)", o.schema)
	}

	generated, err := generateParams(o.schema, o.pkg)
	if err != nil {
		return err
	}
	if o.check {
		committed, rerr := os.ReadFile(o.out)
		if rerr != nil {
			return fmt.Errorf("%s is missing: run `charly candy params %s` to generate it (%w)", o.out, o.name, rerr)
		}
		if string(committed) != string(generated) {
			return fmt.Errorf("%s is STALE: it differs from what %s generates — run `charly candy params %s` and commit the result", o.out, o.schema, o.name)
		}
		fmt.Printf("charly candy params: %s is current\n", o.out)
		return nil
	}
	// Write only on a real change, so a clean regeneration is a true no-op (mtime included).
	if committed, rerr := os.ReadFile(o.out); rerr == nil && string(committed) == string(generated) {
		fmt.Printf("charly candy params: %s already current\n", o.out)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(o.out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(o.out, generated, 0o644); err != nil {
		return err
	}
	fmt.Printf("charly candy params: wrote %s\n", o.out)
	return nil
}

// paramsPipeline IS the schema→Go pipeline — `spec/schemaparams.Generate`, whose contract is
// (schemaDir, pkg): concatenate, compose ONE offline module instance, generate in-process, retag.
// It is a package var so THIS verb's own wiring — the path derivation, the write-on-change no-op,
// the --check comparison — is testable without a schema (params_test.go).
//
// The pipeline's own arms are NOT duplicated here. They live with the pipeline, in
// spec/schemaparams_test.go: byte-identity against the pinned `cue exp gengotypes`, the imported
// definition emitted through its Go home, the package name, the file filter, and the EMPTY schema
// directory (`no *.cue files`). One home, because the behaviour has one implementation; a second copy
// here could only drift from it.
var paramsPipeline = schemaparams.Generate

// generateParams returns the file the schema produces. Generation is in-process inside
// schemaparams.Generate, so this verb and spec's own generator run the same code and there is no
// path by which it could run a different `cue`.
func generateParams(schemaDir, pkg string) ([]byte, error) {
	return paramsPipeline(schemaDir, pkg)
}
