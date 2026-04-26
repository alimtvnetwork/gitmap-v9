package cmd

// Reusable helpers for JSON snapshot tests (extracted from
// startuplistjson_snapshot_test.go to keep that file under the
// 200-line code-style budget). Future `--format=json` snapshot
// tests should land their helpers here too rather than spawning
// per-feature helper files.
//
// Design contract:
//
//   - Token-stream parsing (NOT json.Unmarshal into map[string]any),
//     so on-the-wire key ORDER is observable. A map-based parse
//     would silently lose the very property these tests exist to
//     pin down.
//
//   - Per-object failure messages name the offending object index
//     and the missing/unexpected/reordered key. A schema regression
//     in row 17 of a 100-row output points at row 17, not at the
//     whole blob.
//
//   - No dependency on any specific encoder or schema — these
//     helpers operate on `[]byte` of already-rendered JSON, so they
//     work for stablejson, encoding/json, or any future encoder.

import (
	"bytes"
	"encoding/json"
	"testing"
)

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

// collectObjectKeys reads key-value pairs from an already-opened
// JSON object (the `{` token has already been consumed) and returns
// the keys in wire order. It skips each value with dec.Token() so
// nested structures are handled correctly without recursive logic.
func collectObjectKeys(t *testing.T, dec *json.Decoder) []string {
	t.Helper()
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			t.Fatalf("reading object key: %v", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			t.Fatalf("expected string key, got %v", keyTok)
		}
		keys = append(keys, key)
		if err := skipValue(dec); err != nil {
			t.Fatalf("skipping value for key %q: %v", key, err)
		}
	}
	// consume the closing `}`
	if _, err := dec.Token(); err != nil {
		t.Fatalf("reading closing }: %v", err)
	}
	return keys
}

// skipValue advances the decoder past exactly one JSON value,
// handling scalars (single Token call) and composites (bracket
// matching). This lets collectObjectKeys skip values without
// unmarshalling them into Go types.
func skipValue(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		// scalar — already consumed
		return nil
	}
	var closing json.Delim
	switch delim {
	case '{':
		closing = '}'
	case '[':
		closing = ']'
	default:
		return nil
	}
	for dec.More() {
		if err := skipValue(dec); err != nil {
			return err
		}
		if delim == '{' {
			// also skip the value half of the key:value pair
			if err := skipValue(dec); err != nil {
				return err
			}
		}
	}
	tok, err = dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != closing {
		return errBadDelim(string(closing), tok)
	}
	return nil
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

// equalStringSlices returns true iff a and b have the same length
// and identical elements in the same order. Used by
// assertObjectKeysExactAt to detect key reordering separately from
// missing/unexpected keys so the failure message is precise.
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
