package constants

// Clone progress format strings.
const (
	ProgressBeginFmt   = "[%3d/%d]  Cloning %s ..."
	ProgressDoneFmt    = " done (%s)\n"
	ProgressSkipFmt    = " skipped (cached)\n"
	ProgressFailFmt    = " FAILED\n"
	ProgressSummaryFmt = "\nClone complete: %d/%d repos in %s\n"
	ProgressDetailFmt  = "  Cloned: %d | Pulled: %d | Skipped: %d | Failed: %d\n"
)

// Batch progress format strings (generic operations).
const (
	BatchProgressBeginFmt   = "[%3d/%d]  %s ..."
	BatchProgressDoneFmt    = " done (%s)\n"
	BatchProgressFailFmt    = " FAILED\n"
	BatchProgressSkipFmt    = " skipped\n"
	BatchProgressSummaryFmt = "\n%s complete: %d/%d in %s\n"
	BatchProgressDetailFmt  = "  Succeeded: %d | Failed: %d | Skipped: %d\n"
	BatchStoppedMsg         = "  ⚠ Halted early (--stop-on-fail)\n"
)

// Batch failure report format strings.
const (
	BatchFailureHeader    = "  ── Failed Items ──"
	BatchFailureEntryFmt  = "  %d. %s: %s\n"
	BatchFailureFooterFmt = "  ── %d failure(s) total ──\n"
	ExitPartialFailure    = 3
)

// Batch flag constants.
const (
	FlagStopOnFail     = "stop-on-fail"
	FlagDescStopOnFail = "Stop batch operation after first failure"
)

// Clone shorthands — short aliases for `gitmap clone <source>` that
// expand to the default scan output files (json/csv/text).
const (
	ShorthandJSON = "json"
	ShorthandCSV  = "csv"
	ShorthandText = "text"
)

// Multi-URL clone messages (spec/01-app/104-clone-multi.md).
const (
	MsgCloneInvalidURLFmt    = "  ⚠ Skipping invalid URL: %s\n"
	MsgCloneSummaryMultiFmt  = "\n  Multi-clone summary: %d succeeded, %d failed (of %d URLs)\n"
	MsgCloneRegisteredInline = "  ✓ Registered with GitHub Desktop: %s\n"
	MsgCloneMultiBegin       = "\n  Cloning %d repositories...\n"
	MsgCloneMultiItem        = "\n  [%d/%d] %s\n"
	ErrCloneAllInvalid       = "  ✗ All URLs were invalid — nothing to clone\n"
	ErrCloneMultiFailedFmt   = "  ✗ [%d/%d] %s failed: %v\n"
)

// Multi-clone exit codes.
const (
	ExitCloneMultiPartialFail = 1
	ExitCloneMultiAllInvalid  = 3
)

