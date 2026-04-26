package cmd

// Schema-snapshot test for `gitmap startup-list --format=json`.
//
// This test complements (does NOT replace) the byte-exact golden
// fixtures in startuplistjson_contract_test.go. The goldens catch
// ANY change with a single opaque "bytes differ" message. This file
// catches the three SPECIFIC schema regressions that downstream
// consumers care about, with a targeted failure message for each:
//
//   1. Added field   → "unexpected key X in object[i]"
//   2. Removed field → "missing key X in object[i]"
//   3. Renamed field → both of the above fire on the same object
//   4. Reordered field → "key order drift in object[i]: want ... got ..."
//   5. Non-deterministic encoding → "determinism broken: run N differs"
//
// Why both layers? The golden test is the source of truth for the
// EXACT bytes (catches indentation / trailing-newline drift that no
// schema check would notice). This snapshot test is the source of
// truth for the SCHEMA SHAPE (catches what changed and where, in
// language a downstream JSON consumer would understand). When both
// fail, the goldens get regenerated; when only this fails, the
// schema itself drifted and the consumer-facing changelog must be
// bumped before regenerating.

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/startup"
)

// expectedStartupListJSONSchema pins the EXACT key set and order
// every object in the array must have. Adding a field to
// startupListJSONKey* in startuplistrender.go without also adding it
// here is the regression this test exists to catch.
var expectedStartupListJSONSchema = []string{"name", "path", "exec"}

// TestStartupListJSONSnapshot_SchemaIsLocked is the headline schema
// guarantee. Encodes a representative multi-entry list, then asserts
// every object has EXACTLY the expected keys in EXACTLY the
// expected order. Added / removed / renamed / reordered fields all
// produce a targeted failure message naming the offending object
// index and key.
func TestStartupListJSONSnapshot_SchemaIsLocked(t *testing.T) {
	entries := []startup.Entry{
		{Name: "gitmap-a", Path: "/p/a.desktop", Exec: "/bin/a"},
		{Name: "gitmap-b", Path: "/p/b.desktop", Exec: "/bin/b --flag"},
		// Empty Exec is intentional — past regressions have come
		// from "helpful" omitempty tags hiding zero-value fields.
		// Including this row guarantees the schema check sees the
		// key whether or not the value is the zero string.
		{Name: "gitmap-c", Path: "/p/c.desktop", Exec: ""},
	}
	var buf bytes.Buffer
	if err := encodeStartupListJSON(&buf, entries); err != nil {
		t.Fatalf("encode: %v", err)
	}
	assertEveryObjectKeysExact(t, buf.Bytes(), expectedStartupListJSONSchema)
}

// TestStartupListJSONSnapshot_DeterministicAcrossRuns proves the
// encoder produces byte-identical output when called repeatedly
// with the same input. The most common way determinism breaks is a
// map being introduced into the encoding path (Go map iteration is
// randomized). Running 32 times gives ~1 in 4 billion odds of
// missing a 50/50 ordering bug — high enough confidence for a unit
// test without making the suite slow.
func TestStartupListJSONSnapshot_DeterministicAcrossRuns(t *testing.T) {
	entries := []startup.Entry{
		{Name: "gitmap-x", Path: "/p/x.desktop", Exec: "/bin/x"},
		{Name: "gitmap-y", Path: "/p/y.desktop", Exec: "/bin/y"},
		{Name: "gitmap-z", Path: "/p/z.desktop", Exec: ""},
	}
	const runs = 32
	first := mustEncodeStartupList(t, entries)
	for i := 1; i < runs; i++ {
		got := mustEncodeStartupList(t, entries)
		if !bytes.Equal(first, got) {
			t.Fatalf("determinism broken: run %d differs from run 0\n--- run 0\n%s--- run %d\n%s",
				i, string(first), i, string(got))
		}
	}
}

// TestStartupListJSONSnapshot_EmptyListHasNoObjects guards the edge
// case where the schema check has no objects to inspect — empty
// list MUST still encode as `[]` (verified byte-exactly by the
// golden test), and the schema check's "every object" quantifier
// must not vacuously pass on something malformed.
func TestStartupListJSONSnapshot_EmptyListHasNoObjects(t *testing.T) {
	var buf bytes.Buffer
	if err := encodeStartupListJSON(&buf, nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	keysPerObject := readEveryObjectKeys(t, buf.Bytes())
	if len(keysPerObject) != 0 {
		t.Fatalf("empty list must produce zero objects, got %d: %v",
			len(keysPerObject), keysPerObject)
	}
}

// mustEncodeStartupList is the determinism test's per-run helper.
// Returns a copy of the encoded bytes so the caller can compare
// across runs without aliasing a reused buffer.
func mustEncodeStartupList(t *testing.T, entries []startup.Entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := encodeStartupListJSON(&buf, entries); err != nil {
		t.Fatalf("encode: %v", err)
	}

	return append([]byte(nil), buf.Bytes()...)
}

// assertEveryObjectKeysExact parses `raw` as a top-level JSON array
// and asserts that EVERY object in it has exactly `want` keys, in
// `want` order. Reports per-object failures so a schema regression
// in row 17 of a 100-row output points at row 17, not at the whole
// blob. Uses the same json.Decoder.Token() path as the existing
// assertObjectKeyOrder helper so on-the-wire ordering is checked,
// never a map-shuffled view of it.
func assertEveryObjectKeysExact(t *testing.T, raw []byte, want []string) {
	t.Helper()
	keysPerObject := readEveryObjectKeys(t, raw)
	if len(keysPerObject) == 0 {
		t.Fatalf("expected at least one object, got zero")
	}
	for i, got := range keysPerObject {
		assertObjectKeysExactAt(t, i, got, want)
	}
}

// assertObjectKeysExactAt does the per-object comparison with three
// targeted failure modes (missing, unexpected, reordered) so the
// test output tells the developer exactly what kind of schema drift
// happened. Split out so assertEveryObjectKeysExact stays under the
// 15-line function budget.
func assertObjectKeysExactAt(t *testing.T, idx int, got, want []string) {
	t.Helper()
	wantSet := stringSet(want)
	gotSet := stringSet(got)
	for _, k := range want {
		if !gotSet[k] {
			t.Errorf("missing key %q in object[%d] (got keys %v)", k, idx, got)
		}
	}
	for _, k := range got {
		if !wantSet[k] {
			t.Errorf("unexpected key %q in object[%d] (want keys %v)", k, idx, want)
		}
	}
	if !equalStringSlices(got, want) && len(got) == len(want) {
		t.Errorf("key order drift in object[%d]\n  want: %v\n  got:  %v", idx, want, got)
	}
}

// readEveryObjectKeys streams the entire top-level array and returns
// one []string of keys per object. Returns an empty outer slice for
// `[]` so callers can distinguish "no objects" from "malformed".
func readEveryObjectKeys(t *testing.T, raw []byte) [][]string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := expectArrayStart(dec); err != nil {
		t.Fatalf("expected top-level array: %v", err)
	}
	var out [][]string
	for dec.More() {
		if err := expectObjectStart(dec); err != nil {
			t.Fatalf("expected object at index %d: %v", len(out), err)
		}
		out = append(out, collectObjectKeys(t, dec))
	}

	return out
}

// expectArrayStart reads the next token and confirms it's `[`.
// Factored out so readEveryObjectKeys stays readable and the error
// message stays specific ("expected top-level array").
func expectArrayStart(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return errBadDelim("[", tok)
	}

	return nil
}

// expectObjectStart reads the next token and confirms it's `{`.
// Mirror of expectArrayStart for the per-element check.
func expectObjectStart(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return errBadDelim("{", tok)
	}

	return nil
}

// errBadDelim builds a descriptive error for the expect* helpers.
// Keeps the call sites narrow (single return) and the message
// uniform ("want X, got <type> Y").
func errBadDelim(want string, got any) error {
	return &snapshotErr{want: want, got: got}
}

type snapshotErr struct {
	want string
	got  any
}

func (e *snapshotErr) Error() string {
	return "want delim " + e.want + ", got " + sprintToken(e.got)
}

// sprintToken renders a json.Token in a form that distinguishes
// `json.Delim('{')` from the literal string "{". Without this, a
// schema test failing on an unexpected scalar would print the
// scalar with no type info and the developer would have to guess.
func sprintToken(tok any) string {
	switch v := tok.(type) {
	case json.Delim:
		return "delim " + string(v)
	case string:
		return "string " + v
	case nil:
		return "nil"
	}

	return "unknown token"
}

// stringSet is the tiny set helper used by the missing/unexpected
// passes. Inline construction keeps the schema check zero-allocation
// on the success path (Go maps with <8 entries fit in one bucket).
func stringSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}

	return m
}
