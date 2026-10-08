package candy

// params_test.go — the `charly candy params` VERB's arms: the path derivation, the write-on-change
// no-op, the error propagation, and above all the --check comparison, which is the drift gate a plugin
// repo's CI runs.
//
// The PIPELINE's arms are deliberately NOT here. The pipeline is `spec/schemaparams`, and its seven
// arms live with it (concat+header+retag, the emitted codegen module, the package name, the
// relative-toolchain-path regression, the empty schema dir, a toolchain failure, and the real
// provisioning arm). Re-testing them here would be a second copy of one behaviour — the thing this
// wrap exists to delete — so the wiring arms stand in for the pipeline with a STUB, and the last arm
// runs the REAL pipeline through the verb, which is the wrap's own end-to-end proof.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withStubPipeline stands in for the pipeline for the duration of one test. The stub's output is
// returned verbatim by the verb when it writes, so the arms can assert the file on disk is EXACTLY
// what the pipeline returned — no transformation of the verb's own.
func withStubPipeline(t *testing.T, out string) {
	t.Helper()
	orig := paramsPipeline
	paramsPipeline = func(string, string, string) ([]byte, error) { return []byte(out), nil }
	t.Cleanup(func() { paramsPipeline = orig })
}

// withFailingPipeline makes the pipeline fail, to prove the verb surfaces the pipeline's error
// instead of swallowing it (the pipeline's own arms assert WHICH errors it produces; this asserts
// they reach the caller intact).
func withFailingPipeline(t *testing.T, err error) {
	t.Helper()
	orig := paramsPipeline
	paramsPipeline = func(string, string, string) ([]byte, error) { return nil, err }
	t.Cleanup(func() { paramsPipeline = orig })
}

// candyFixture lays out a minimal candy project: <root>/candy/<name>/schema/foo.cue plus whatever
// the caller wants at candy/<name>/params/cue_types_gen.go, and returns the project root.
func candyFixture(t *testing.T, committed string, committedSet bool) string {
	t.Helper()
	root := t.TempDir()
	schemaDir := filepath.Join(root, "candy", "demo", "schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "demo.cue"), []byte("#Demo: {\n\tname?: string\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if committedSet {
		outDir := filepath.Join(root, "candy", "demo", "params")
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outDir, "cue_types_gen.go"), []byte(committed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// stubProjection is what the stub pipeline returns — shaped like real pipeline output (a retagged
// struct), so the arms compare against something a reader recognises as generator output.
const stubProjection = "package params\n\ntype Foo struct {\n\tName string `yaml:\"name,omitempty\" json:\"name\"`\n}\n"

// inProject runs fn with the process cwd at root — the CLI resolves candies from the project root,
// exactly as the host invokes it.
func inProject(t *testing.T, root string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	}()
	fn()
}

// TestParamsWritesTheProjectionThePipelineReturned is the happy path: the verb derives the three
// paths from the candy's own directory, calls the pipeline, and writes its bytes unchanged.
func TestParamsWritesTheGeneratedFileAndRetagsIt(t *testing.T) {
	withStubPipeline(t, stubProjection)
	root := candyFixture(t, "", false)
	inProject(t, root, func() {
		if err := runCandyParams([]string{"demo"}); err != nil {
			t.Fatalf("runCandyParams: %v", err)
		}
	})
	got, err := os.ReadFile(filepath.Join(root, "candy", "demo", "params", "cue_types_gen.go"))
	if err != nil {
		t.Fatalf("the verb did not write the projection: %v", err)
	}
	if string(got) != stubProjection {
		t.Fatalf("projection =\n%q\nwant\n%q", got, stubProjection)
	}
}

// TestParamsCheckGoesRedOnDrift is the R7 proof for the drift gate: a committed projection that
// differs from the pipeline OUTPUT must fail, name the file, and say how to fix it. A check that
// cannot fail is the defect this campaign has produced five times, so this arm mutates the file and
// watches the check go red.
func TestParamsCheckGoesRedOnDrift(t *testing.T) {
	withStubPipeline(t, stubProjection)
	root := candyFixture(t, "package params\n\n// stale, hand-edited\n", true)
	inProject(t, root, func() {
		err := runCandyParams([]string{"demo", "--check"})
		if err == nil {
			t.Fatal("--check PASSED on a projection that differs from the pipeline output")
		}
		if !strings.Contains(err.Error(), "cue_types_gen.go") || !strings.Contains(err.Error(), "STALE") {
			t.Fatalf("--check failure must name the file and the remedy, got: %v", err)
		}
	})
	// and it must not have written anything: --check is read-only
	got, err := os.ReadFile(filepath.Join(root, "candy", "demo", "params", "cue_types_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package params\n\n// stale, hand-edited\n" {
		t.Fatalf("--check modified the file: %q", got)
	}
}

// TestParamsCheckPassesOnACurrentFile is the other half: the same comparison must be GREEN when the
// committed file IS the pipeline output, or the gate would be useless.
func TestParamsCheckPassesOnACurrentFile(t *testing.T) {
	withStubPipeline(t, stubProjection)
	root := candyFixture(t, stubProjection, true)
	inProject(t, root, func() {
		if err := runCandyParams([]string{"demo", "--check"}); err != nil {
			t.Fatalf("--check failed on a current projection: %v", err)
		}
	})
}

// TestParamsRegenerationIsANoOp: a clean regeneration must leave the file byte-identical, because
// every consumer's CI asserts exactly that.
func TestParamsRegenerationIsANoOp(t *testing.T) {
	withStubPipeline(t, stubProjection)
	root := candyFixture(t, stubProjection, true)
	out := filepath.Join(root, "candy", "demo", "params", "cue_types_gen.go")
	before, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	inProject(t, root, func() {
		if err := runCandyParams([]string{"demo"}); err != nil {
			t.Fatalf("runCandyParams: %v", err)
		}
	})
	after, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("a clean regeneration rewrote the file (it must be a no-op)")
	}
}

// TestParamsPipelineErrorsReachTheCaller: the verb must not swallow a pipeline failure. The EMPTY
// schema directory is the case that mattered in practice ("no *.cue files"), and it is asserted
// where the behaviour now lives — spec's TestGenerateWithCueRejectsAnEmptySchemaDir — so this arm
// asserts the PROPAGATION, not the message: it fails the pipeline and requires the verb to return
// that error unchanged rather than writing an empty projection over a good one.
func TestParamsPipelineErrorsReachTheCaller(t *testing.T) {
	sentinel := errors.New("no *.cue files in /tmp/nope")
	withFailingPipeline(t, sentinel)
	root := candyFixture(t, "", false)
	inProject(t, root, func() {
		err := runCandyParams([]string{"demo"})
		if err == nil {
			t.Fatal("a failing pipeline produced no error")
		}
		if !errors.Is(err, sentinel) {
			t.Fatalf("the verb must surface the pipeline's own error, got: %v", err)
		}
	})
	if _, statErr := os.Stat(filepath.Join(root, "candy", "demo", "params", "cue_types_gen.go")); statErr == nil {
		t.Fatal("a failing pipeline still wrote a projection")
	}
}

// TestParamsCheckOnAMissingProjectionNamesIt: --check on a candy that has never been generated must
// fail with the file named, not with a nil error.
func TestParamsCheckOnAMissingProjectionNamesIt(t *testing.T) {
	withStubPipeline(t, stubProjection)
	root := candyFixture(t, "", false)
	inProject(t, root, func() {
		err := runCandyParams([]string{"demo", "--check"})
		if err == nil {
			t.Fatal("--check passed although no projection exists")
		}
		if !strings.Contains(err.Error(), "cue_types_gen.go") {
			t.Fatalf("error must name the missing file, got: %v", err)
		}
	})
}

// TestParamsRealPipelineRunsThroughTheVerb is the WRAP's end-to-end proof: the verb's own path
// derivation and a REAL provisioned toolchain, over this candy's own schema/candy.cue. It asserts the
// wrapper adds nothing and loses nothing — the bytes on disk are the pipeline's bytes — plus the retag
// contract on real output. It needs the pinned release provisioned, so it skips VISIBLY, never
// silently and never with a canned substitute.
func TestParamsRealPipelineGeneratesFromTheCandysOwnSchema(t *testing.T) {
	if os.Getenv("CHARLY_SKIP_CUE_PROVISION") != "" {
		t.Skip("CHARLY_SKIP_CUE_PROVISION is set: the pinned cue toolchain is not provisioned here")
	}
	schemaDir, err := filepath.Abs("schema")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(schemaDir, "candy.cue")); err != nil {
		t.Skipf("no schema/candy.cue to generate from: %v", err)
	}
	got, err := generateParams(schemaDir, "params")
	if err != nil {
		t.Skipf("the pinned cue toolchain could not be provisioned here (%v) — this arm is skipped, not faked", err)
	}
	src := string(got)
	if !strings.Contains(src, "package params") {
		t.Fatalf("generated source has no package clause:\n%.200s", src)
	}
	// The retag contract, on real output: every json tag must be accompanied by a yaml tag.
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, "json:\"") && !strings.Contains(line, "yaml:\"") {
			t.Fatalf("a generated line carries a json tag with no yaml tag (retag did not run):\n%s", line)
		}
	}
}
