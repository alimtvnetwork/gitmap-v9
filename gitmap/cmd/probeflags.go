package cmd

// probeflags.go — argument parsing for `gitmap probe`.
//
// Split out of probe.go to honor the 200-line per-file budget. The
// dispatcher (runProbe) lives in probe.go and consumes probeOptions
// produced here. parseProbeArgs is order-agnostic and supports both
// `--workers N` and `--workers=N` forms so the flag composes with the
// existing positional arg slot (a repo path or `--all`).

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// probeOptions captures the parsed CLI flags for `gitmap probe`.
// Workers is already clamped into [1, ProbeMaxWorkers] by parseProbeArgs.
type probeOptions struct {
	jsonOut bool
	workers int
	rest    []string
}

// parseProbeArgs walks the arg list, peeling off recognised flags and
// returning everything else as positional args. Order-agnostic; supports
// both `--workers N` and `--workers=N`.
func parseProbeArgs(args []string) (probeOptions, error) {
	opts := probeOptions{workers: constants.ProbeDefaultWorkers, rest: make([]string, 0, len(args))}
	for i := 0; i < len(args); i++ {
		next, consumed, err := applyProbeFlag(&opts, args, i)
		if err != nil {
			return opts, err
		}
		if !consumed {
			opts.rest = append(opts.rest, args[i])
		}
		i = next
	}

	return opts, nil
}

// applyProbeFlag inspects args[i] and applies it to opts when it
// matches a known flag. Returns the new loop index (which may equal i
// or i+1 for two-arg flags) and a `consumed` bool — false means the
// caller should treat args[i] as positional.
func applyProbeFlag(opts *probeOptions, args []string, i int) (int, bool, error) {
	a := args[i]
	if a == constants.ProbeFlagJSON {
		opts.jsonOut = true
		return i, true, nil
	}
	if a == constants.ProbeFlagWorkers {
		return applyWorkersTwoArg(opts, args, i)
	}
	if next, ok, err := applyWorkersInline(opts, a, i); ok || err != nil {
		return next, true, err
	}

	return i, false, nil
}

// applyWorkersTwoArg handles `--workers N` (value is the next arg).
func applyWorkersTwoArg(opts *probeOptions, args []string, i int) (int, bool, error) {
	if i+1 >= len(args) {
		return i, true, fmt.Errorf(constants.ErrProbeWorkersMissing)
	}
	n, err := parseWorkersValue(args[i+1])
	if err != nil {
		return i, true, err
	}
	opts.workers = clampProbeWorkers(n)

	return i + 1, true, nil
}

// applyWorkersInline handles `--workers=N`. ok=false means this token
// is not a `--workers=` form and the caller should keep matching.
func applyWorkersInline(opts *probeOptions, token string, i int) (int, bool, error) {
	prefix := constants.ProbeFlagWorkers + "="
	if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
		return i, false, nil
	}
	n, err := parseWorkersValue(token[len(prefix):])
	if err != nil {
		return i, true, err
	}
	opts.workers = clampProbeWorkers(n)

	return i, true, nil
}

// parseWorkersValue validates the `--workers` argument as a positive int.
func parseWorkersValue(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf(constants.ErrProbeWorkersValue, s)
	}

	return n, nil
}

// clampProbeWorkers enforces the [1, ProbeMaxWorkers] cap, printing a
// notice to stderr when the user asked for more than we'll grant.
func clampProbeWorkers(n int) int {
	if n > constants.ProbeMaxWorkers {
		fmt.Fprintf(os.Stderr, constants.MsgProbeWorkersClamped, n, constants.ProbeMaxWorkers)
		return constants.ProbeMaxWorkers
	}

	return n
}
