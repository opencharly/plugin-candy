package candy

// params.go — `charly candy params <candy>`: regenerate a plugin candy's Go params from its own
// `candy/<name>/schema/*.cue`, using NOTHING but the charly binary.
//
// WHY THIS EXISTS (R4a, opencharly/charly#829): a plugin candy's `schema/*.cue` is a PUBLISHED
// surface — `charly docs generate` renders it into `reference/plugin/<plugin>/plugin-*.md` — and a
// published page may only name commands that work with the `charly` binary alone. Before this verb
// the only way to regenerate was a checkout-relative recipe (`cd` into a spec checkout, run
// `internal/schemagen -mode=concat`, run `cue exp gengotypes`, run `-mode=retag`), so the pages had
// to publish either a forbidden path or nothing. The verb provisions its own toolchain instead: the
// pinned `cue` CLI is fetched checksum-verified into the charly cache, and the pipeline runs from
// the schema's own declarations.
//
// THE PIPELINE, and where each step's contract lives:
//  1. concatenate candy/<name>/schema/*.cue           -> spec/schemaconcat  (the ONE concat contract)
//  2. head it `package params` + `@go(params)`         -> this file (the header IS the contract)
//  3. `cue exp gengotypes`                             -> cuetoolchain (the ONE pin)
//  4. double every json tag with a yaml tag            -> spec/schemaretag  (the ONE retag contract)
//  5. write candy/<name>/params/cue_types_gen.go       -> this file
//
// --check compares the COMMITTED file against the pipeline's OUTPUT (never a grep, and never a hash
// of a previous run): that is the only comparison that can catch a generated file gone stale, which
// is why a plugin repo's CI can run `charly candy params <name> --check` as its drift gate.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/cuetoolchain"
	"github.com/opencharly/spec/schemaconcat"
	"github.com/opencharly/spec/schemaretag"
)

const paramsUsage = `usage: charly candy params <name> [--check]
       charly candy params --schema <dir> --pkg <params> --out <file> [--check]

Regenerate a plugin candy's Go params from its own schema/*.cue — nothing but the
charly binary is needed: the pinned cue CLI (` + cuetoolchain.Version + `, checksum-verified) is
provisioned into the charly cache on first use.

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

// cueBinary resolves the pinned toolchain. It is a package var so the pipeline's wiring — the
// concat, the header, the retag, the write and the --check comparison — can be tested with a stub
// binary (params_test.go) without provisioning the release; the REAL resolver is exercised by
// TestParamsRealPipelineGeneratesFromTheCandysOwnSchema, which skips visibly when the toolchain
// cannot be provisioned.
var cueBinary = ensureCue

// generateParams runs the whole pipeline and returns the file the schema produces.
func generateParams(schemaDir, pkg string) ([]byte, error) {
	body, files, err := schemaconcat.ConcatSchema(os.DirFS(schemaDir), ".", nil)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", schemaDir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no *.cue files in %s", schemaDir)
	}
	cueBin, err := cueBinary()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "charly-candy-params-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	// The header is the concat contract's tail: `package <pkg>` + the file-level @go(<pkg>)
	// attribute gengotypes reads to name the Go package.
	src := "package " + pkg + "\n\n@go(" + pkg + ")\n\n" + body
	if err := os.WriteFile(filepath.Join(tmp, pkg+".cue"), []byte(src), 0o644); err != nil {
		return nil, err
	}
	cmd := exec.Command(cueBin, "exp", "gengotypes", ".")
	cmd.Dir = tmp
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("cue exp gengotypes failed in %s: %w\n%s", tmp, err, strings.TrimSpace(string(out)))
	}
	gen, err := os.ReadFile(filepath.Join(tmp, "cue_types_"+pkg+"_gen.go"))
	if err != nil {
		return nil, fmt.Errorf("gengotypes produced no cue_types_%s_gen.go: %w", pkg, err)
	}
	return schemaretag.Normalize(gen), nil
}

// cueCacheDir is where the provisioned toolchain lives: one directory per pinned version, so a pin
// bump cannot be served a stale binary from the previous one.
func cueCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "charly", "cue", cuetoolchain.Version), nil
}

// ensureCue returns a path to the pinned `cue` CLI, provisioning it on first use. The pin, its
// checksum, the arch rules, the verification-BEFORE-extraction and the atomic write all live in
// spec/cuetoolchain — the ONE home for the toolchain, shared with spec's own bootstrap-cue task — so
// this verb cannot run a different cue than the module's generator does. Every failure is loud and
// names the pin and the remedy: never a silent fallback to whatever `cue` is on PATH.
func ensureCue() (string, error) {
	dir, err := cueCacheDir()
	if err != nil {
		return "", err
	}
	fmt.Printf("charly candy params: ensuring %s in %s\n", cuetoolchain.Pin(), dir)
	bin, err := cuetoolchain.Ensure(dir)
	if err != nil {
		return "", fmt.Errorf("charly candy params: %w", err)
	}
	return bin, nil
}
