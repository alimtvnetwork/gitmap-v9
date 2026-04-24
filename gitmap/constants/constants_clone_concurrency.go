package constants

// Constants for parallel clone execution (`gitmap clone --max-concurrency N`).
//
// The flag is opt-in: the default of 1 keeps the historical sequential
// behavior so deterministic ordering of stderr progress lines is
// preserved for users who expect it. Setting N>1 dispatches the
// per-record clone work onto a bounded worker pool (see
// gitmap/cloner/concurrent.go); the on-disk nested folder hierarchy
// remains identical regardless of N because every worker still uses
// each ScanRecord.RelativePath verbatim.

// CloneFlagMaxConcurrency is the long-form flag name (`--max-concurrency`)
// that controls the worker-pool size for `gitmap clone`.
const CloneFlagMaxConcurrency = "max-concurrency"

// CloneDefaultMaxConcurrency is the default worker count: 1 means
// sequential, byte-for-byte compatible with the pre-v3.101 behavior.
const CloneDefaultMaxConcurrency = 1

// FlagDescCloneMaxConcurrency is the help text shown by `gitmap help clone`.
const FlagDescCloneMaxConcurrency = "Run up to N clones in parallel (1 = sequential, the default). Hierarchy is preserved at any N."

// MsgCloneConcurrencyEnabledFmt is printed once before the first
// progress line when the parallel runner takes over. Keeps a single,
// stable line that scripts can grep for.
const MsgCloneConcurrencyEnabledFmt = "  ↪ parallel clone enabled: %d workers\n"

// ErrCloneMaxConcurrencyInvalid is printed when the user supplies a
// non-positive integer to --max-concurrency. The CLI exits 1 to keep
// the contract: invalid input never silently degrades to a default.
const ErrCloneMaxConcurrencyInvalid = "clone --max-concurrency: must be a positive integer (got %d)\n"
