package candy

// params_test.go — the `charly candy params` pipeline's arms: the concat+header, the retag, the
// write-on-change, and above all the --check comparison, which is the drift gate a plugin repo's CI
// runs. The wiring arms run against a STUB cue binary (deterministic, no network); the real
// toolchain and the real schema are exercised by the last test, which skips VISIBLY when the pinned
// release cannot be provisioned.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubCue writes an executable that stands in for `cue exp gengotypes`: it emits a fixed file in its
// working directory, with a json-ONLY tag so the retag step has something to do.
func stubCue(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "cue")
	script := "#!/bin/sh\ncat > cue_types_params_gen.go <<'EOF'\n" + body + "\nEOF\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// withStubCue swaps the toolchain resolver for the duration of one test.
func withStubCue(t *testing.T, body string) {
	t.Helper()
	orig := cueBinary
	cueBinary = func() (string, error) { return stubCue(t, body), nil }
	t.Cleanup(func() { cueBinary = orig })
}

// stubBody deliberately ends WITHOUT a trailing newline: the stub heredoc supplies one, so the
// file the pipeline reads is the body plus exactly one newline (what gengotypes emits).
const stubBody = "package params\n\ntype Foo struct {\n\tName string `json:\"name\"`\n}"

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

// expectedPipelineOutput is what the stub pipeline must produce for stubBody: the retag doubles the
// json-only tag with a yaml tag and then adds ,omitempty to that bare yaml key.
const expectedPipelineOutput = "package params\n\ntype Foo struct {\n\tName string `yaml:\"name,omitempty\" json:\"name\"`\n}\n"

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

// TestParamsWritesTheGeneratedFileAndRetagsIt is the happy path: the pipeline concatenates the
// candy's schema, runs the toolchain, normalizes the tags and writes the projection.
func TestParamsWritesTheGeneratedFileAndRetagsIt(t *testing.T) {
	withStubCue(t, stubBody)
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
	if string(got) != expectedPipelineOutput {
		t.Fatalf("projection =\n%q\nwant\n%q", got, expectedPipelineOutput)
	}
}

// TestParamsCheckGoesRedOnDrift is the R7 proof for the drift gate: a committed projection that
// differs from the pipeline OUTPUT must fail, name the file, and say how to fix it. A check that
// cannot fail is the defect this campaign has produced five times, so this arm mutates the file and
// watches the check go red.
func TestParamsCheckGoesRedOnDrift(t *testing.T) {
	withStubCue(t, stubBody)
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
	withStubCue(t, stubBody)
	root := candyFixture(t, expectedPipelineOutput, true)
	inProject(t, root, func() {
		if err := runCandyParams([]string{"demo", "--check"}); err != nil {
			t.Fatalf("--check failed on a current projection: %v", err)
		}
	})
}

// TestParamsRegenerationIsANoOp: a clean regeneration must leave the file byte-identical, because
// every consumer's CI asserts exactly that.
func TestParamsRegenerationIsANoOp(t *testing.T) {
	withStubCue(t, stubBody)
	root := candyFixture(t, expectedPipelineOutput, true)
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

// TestParamsEmptySchemaDirFailsLoudly: a candy with no schema must say so, not generate an empty file.
func TestParamsEmptySchemaDirFailsLoudly(t *testing.T) {
	withStubCue(t, stubBody)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "candy", "demo", "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	inProject(t, root, func() {
		err := runCandyParams([]string{"demo"})
		if err == nil {
			t.Fatal("an empty schema directory produced no error")
		}
		if !strings.Contains(err.Error(), "no *.cue files") {
			t.Fatalf("error must say the schema dir is empty, got: %v", err)
		}
	})
}

// TestParamsCheckOnAMissingProjectionNamesIt: --check on a candy that has never been generated must
// fail with the file named, not with a nil error.
func TestParamsCheckOnAMissingProjectionNamesIt(t *testing.T) {
	withStubCue(t, stubBody)
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

// TestParamsRealPipelineGeneratesFromTheCandysOwnSchema runs the REAL pinned toolchain over this
// candy's own schema/candy.cue. It proves the end-to-end pipeline (concat + header + gengotypes +
// retag) rather than the wiring alone. It needs the pinned release to be provisioned, so it skips
// VISIBLY — never silently, and never with a canned substitute — when that is impossible.
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
