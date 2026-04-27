package goldenguard

// Tests for the determinism pre-check. These cover three axes:
//   1. Deterministic writer => AssertWriterDeterministic passes.
//   2. Non-deterministic writer => AssertWriterDeterministic fatals.
//   3. AllowUpdateAfterDeterminism short-circuits cleanly when the
//      trigger is off, runs determinism + gate when on, and refuses
//      to consult the gate when determinism fails.
//
// Following the existing pattern in goldenguard_test.go, t.Fatalf
// from the helpers is captured by running them inside a sub-test
// whose Failed() flag we inspect. A direct *testing.T cannot be
// safely faulted from the outside.

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestAssertWriterDeterministic_StableWriter_Passes(t *testing.T) {
	writer := func() ([]byte, error) { return []byte("stable-bytes"), nil }
	if expectFailure(t, func(tt *testing.T) {
		AssertWriterDeterministic(tt, "stable", writer)
	}) {
		t.Fatalf("stable writer must NOT trigger a determinism failure")
	}
}

func TestAssertWriterDeterministic_DriftingWriter_Fails(t *testing.T) {
	var counter int32
	writer := func() ([]byte, error) {
		n := atomic.AddInt32(&counter, 1)
		return []byte{byte(n)}, nil
	}
	if !expectFailure(t, func(tt *testing.T) {
		AssertWriterDeterministic(tt, "drifting", writer)
	}) {
		t.Fatalf("drifting writer must trigger a determinism failure")
	}
}

func TestAssertWriterDeterministic_ErroringWriter_Fails(t *testing.T) {
	writer := func() ([]byte, error) { return nil, errors.New("boom") }
	if !expectFailure(t, func(tt *testing.T) {
		AssertWriterDeterministic(tt, "erroring", writer)
	}) {
		t.Fatalf("erroring writer must trigger a failure (cannot be deterministic)")
	}
}

func TestAllowUpdateAfterDeterminism_TriggerOff_NeverInvokesWriter(t *testing.T) {
	var calls int32
	writer := func() ([]byte, error) {
		atomic.AddInt32(&calls, 1)
		return []byte("x"), nil
	}
	if AllowUpdateAfterDeterminism(t, false, "off", writer) {
		t.Fatalf("trigger=false must return false")
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("writer must not be invoked when trigger=false; got %d calls", got)
	}
}

func TestAllowUpdateAfterDeterminism_DriftingWriter_FailsBeforeGate(t *testing.T) {
	t.Setenv(AllowUpdateEnv, allowUpdateValue) // gate would say YES
	var counter int32
	writer := func() ([]byte, error) {
		n := atomic.AddInt32(&counter, 1)
		return []byte{byte(n)}, nil
	}
	if !expectFailure(t, func(tt *testing.T) {
		AllowUpdateAfterDeterminism(tt, true, "drifting", writer)
	}) {
		t.Fatalf("drifting writer must block the gate even when env allows update")
	}
}

func TestAllowUpdateAfterDeterminism_StableWriterGateOn_ReturnsTrue(t *testing.T) {
	t.Setenv(AllowUpdateEnv, allowUpdateValue)
	writer := func() ([]byte, error) { return []byte("ok"), nil }
	if !AllowUpdateAfterDeterminism(t, true, "stable", writer) {
		t.Fatalf("stable writer + gate ON must return true")
	}
}

// expectFailure runs body inside a sub-test and reports whether it
// recorded a fatal/failure. Mirrors the helper pattern already used
// in goldenguard_test.go (expectFatal) so contributors learn one
// idiom for capturing t.Fatalf in this package.
func expectFailure(parent *testing.T, body func(*testing.T)) bool {
	parent.Helper()
	var failed bool
	parent.Run("captured", func(tt *testing.T) {
		defer func() { _ = recover() }() // t.Fatalf inside helpers may panic via FailNow
		body(tt)
		failed = tt.Failed()
	})
	return failed
}
