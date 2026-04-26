package cmd

// Shared helpers for JSON schema contract tests. Used by:
//
//   - startuplistjson_contract_test.go
//   - latestbranchjson_contract_test.go
//   - findnextjson_contract_test.go
//
// The point of these tests is to make output drift LOUD: any rename,
// reorder, addition, or removal of a JSON field that downstream
// consumers depend on must fail CI before it ships. Two complementary
// strictness levels are offered:
//
//  1. assertGoldenBytes — byte-exact comparison against a committed
//     fixture under testdata/. Catches whitespace, indentation, key
//     order, AND field changes. Used for canonical fixtures (empty
//     list, single hand-authored entry) where the exact output is a
//     deliberate publication.
//
//  2. assertObjectKeyOrder / assertObjectKeyExactSet — structural
//     checks that pin the schema (which keys exist, in what
//     declaration order) without locking down the values. Used for
//     fixtures with variable / time-dependent data so the test
//     stays useful when an underlying field's display format
//     changes for a non-breaking reason.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goldenDir is the conventional Go location for committed test
// fixtures — the `testdata/` directory is ignored by `go build` and
// `go vet`, so files there are test-only.
const goldenDir = "testdata"

// assertGoldenBytes byte-compares `got` against the committed file
// `testdata/<name>`. On mismatch it prints a unified-style diff so
// the failure is actionable. To regenerate intentional fixture
// changes, set GITMAP_UPDATE_GOLDEN=1 in the environment and re-run
// the test — the file will be overwritten with `got`.
func assertGoldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if os.Getenv("GITMAP_UPDATE_GOLDEN") == "1" {
		mustWriteGolden(t, path, got)

		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with GITMAP_UPDATE_GOLDEN=1 to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch for %s\n--- want\n%s\n--- got\n%s",
			name, string(want), string(got))
	}
}

// mustWriteGolden writes a regenerated fixture and fails the test
// loudly so a CI run can never silently pass on a regenerate.
func mustWriteGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	if err := os.WriteFile(path, got, 0o644); err != nil {
		t.Fatalf("write golden %s: %v", path, err)
	}
	t.Fatalf("regenerated golden %s — re-run without GITMAP_UPDATE_GOLDEN to confirm", path)
}

// assertObjectKeyOrder parses `raw` as a JSON object (or as the
// first object inside a top-level array — common for our list
// outputs) and asserts the top-level keys appear in EXACTLY the
// expected order. Goes through json.Decoder.Token() so the actual
// on-the-wire ordering is checked, not a map-shuffled view of it.
func assertObjectKeyOrder(t *testing.T, raw []byte, want []string) {
	t.Helper()
	got := readFirstObjectKeys(t, raw)
	if !equalStringSlices(got, want) {
		t.Fatalf("top-level key order drift\n  want: %v\n  got:  %v", want, got)
	}
}

// readFirstObjectKeys streams tokens from `raw` and returns the keys
// of the first top-level object encountered. Handles both shapes:
//
//   - {...}             → keys of the object itself.
//   - [{...}, ...]      → keys of the first object in the array.
//
// Anything else (number, string, null, bool, empty array) returns
// an empty slice so the caller's comparison fails with a clear
// "got: []" message rather than a panic.
func readFirstObjectKeys(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := skipUntilFirstObjectStart(dec); err != nil {

		return nil
	}

	return collectObjectKeys(t, dec)
}

// skipUntilFirstObjectStart advances the decoder past wrapper tokens
// (top-level `[`) until it reaches the `{` that opens the first
// object. Returns io.EOF (or similar) if no object is found.
func skipUntilFirstObjectStart(dec *json.Decoder) error {
	for {
		tok, err := dec.Token()
		if err != nil {

			return err
		}
		if delim, ok := tok.(json.Delim); ok && delim == '{' {

			return nil
		}
	}
}

// collectObjectKeys reads alternating key / value tokens from an
// object that's already been opened, returning just the keys in the
// order they were emitted. Stops at the matching `}`.
func collectObjectKeys(t *testing.T, dec *json.Decoder) []string {
	t.Helper()
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode token: %v", err)
		}
		key, ok := tok.(string)
		if !ok {
			t.Fatalf("expected string key, got %T (%v)", tok, tok)
		}
		keys = append(keys, key)
		// Skip the value (which might itself be an object/array we
		// don't want to descend into for top-level key checks).
		if err := skipOneValue(dec); err != nil {
			t.Fatalf("skip value for key %q: %v", key, err)
		}
	}

	return keys
}

// skipOneValue reads exactly one JSON value (scalar, object, or
// array) so the decoder is positioned at the next key. Uses
// json.RawMessage as the cheapest "consume one value" primitive in
// the stdlib — it parses the value but discards the AST.
func skipOneValue(dec *json.Decoder) error {
	var raw json.RawMessage

	return dec.Decode(&raw)
}

// equalStringSlices compares slices with an explicit length-then-
// element loop. Faster than reflect.DeepEqual on the test hot path
// and the failure message in the caller is more actionable than
// reflect's auto-generated one.
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {

		return false
	}
	for i := range a {
		if a[i] != b[i] {

			return false
		}
	}

	return true
}

// trimTrailingNewline normalizes encoder output for byte comparison.
// json.Encoder.Encode always appends a trailing '\n'; some golden
// files may omit it. This helper lets either form match.
func trimTrailingNewline(b []byte) []byte {
	return []byte(strings.TrimRight(string(b), "\n"))
}
