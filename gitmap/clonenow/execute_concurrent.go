package clonenow

// execute_concurrent.go — bounded worker-pool variant of
// ExecuteWithHooks. Used by the cmd layer when --max-concurrency
// resolves to >1.
//
// Design contract:
//
//   - The on-disk layout is unchanged: every worker resolves its
//     destination via the row's RelativePath verbatim (same as the
//     sequential path), so increasing the worker count NEVER
//     reshuffles where repos land.
//   - Result ORDER matches input order. We keep stable ordering
//     because the renderer/summary downstream is index-aware and
//     scripts that grep stderr for "[i/total]" expect consistent
//     numbering. Per-row PROGRESS LINES, however, fire in
//     completion order — that's the only observable difference
//     vs. the sequential runner.
//   - The BeforeRow hook is dispatched on the dispatcher goroutine
//     BEFORE the row enters the work queue. Mirrors the sequential
//     hook timing ("before clone starts") with the caveat that
//     "starts" now means "enqueued" rather than "shell-out begins".
//     The cmd layer's standardized terminal block is positional, not
//     time-sensitive, so this is fine.
//   - A workers value <= 1 falls back to the sequential
//     ExecuteWithHooks so there is exactly one code path per regime
//     (no "concurrent runner with N=1" middle ground that drifts).

import (
	"io"
	"os"
	"sync"
)

// ExecuteWithHooksConcurrent is the parallel sibling of
// ExecuteWithHooks. See file header for the contract.
func ExecuteWithHooksConcurrent(plan Plan, cwd string, progress io.Writer,
	beforeRow BeforeRowHook, workers int) []Result {
	if workers <= 1 {
		return ExecuteWithHooks(plan, cwd, progress, beforeRow)
	}
	if len(cwd) == 0 {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	total := len(plan.Rows)
	out := make([]Result, total)
	dispatchConcurrent(plan, cwd, beforeRow, workers, out)
	emitProgressInOrder(progress, out)

	return out
}

// dispatchConcurrent runs the worker pool and fills `out` at each
// row's input index. Split out so ExecuteWithHooksConcurrent stays
// under the 15-line function cap.
func dispatchConcurrent(plan Plan, cwd string, beforeRow BeforeRowHook,
	workers int, out []Result) {
	type job struct {
		idx int
		row Row
	}
	jobs := make(chan job, len(plan.Rows))
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				out[j.idx] = executeRow(j.row, plan, cwd)
			}
		}()
	}
	enqueueClonenowJobsCh(plan, beforeRow, jobs)
	close(jobs)
	wg.Wait()
}

// enqueueClonenowJobsCh fires the BeforeRow hook (synchronously, in
// input order) and enqueues each row. The channel's buffer is
// sized to the row count so this never blocks the dispatcher.
func enqueueClonenowJobsCh(plan Plan, beforeRow BeforeRowHook,
	jobs chan<- struct {
		idx int
		row Row
	}) {
	total := len(plan.Rows)
	for i, r := range plan.Rows {
		if beforeRow != nil {
			url := r.PickURL(plan.Mode)
			beforeRow(i+1, total, r, url, r.RelativePath)
		}
		jobs <- struct {
			idx int
			row Row
		}{idx: i, row: r}
	}
}

// emitProgressInOrder prints progress lines in input order AFTER
// the pool drains. Trade-off: progress is post-hoc rather than
// real-time, but ordering matches the sequential runner's contract
// — keeping `[i/total]` lines monotonic for scripts.
func emitProgressInOrder(w io.Writer, out []Result) {
	if w == nil {
		return
	}
	total := len(out)
	for i, res := range out {
		writeProgress(w, i+1, total, res)
	}
}
