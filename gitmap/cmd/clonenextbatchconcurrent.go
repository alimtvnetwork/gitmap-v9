package cmd

// Bounded worker-pool path for `gitmap cn --all` / `--csv` batch mode.
//
// Split out of clonenextbatch.go to keep that file focused on the
// dispatcher + sequential path and to honor the project's 200-line
// per-file budget. The pattern mirrors gitmap/cloner/concurrent.go:
// workers drain a buffered job channel, write outcomes to a buffered
// result channel, and the collector preserves input ordering so the
// CSV report rows are deterministic regardless of worker count.

// normalizeBatchWorkers clamps the requested worker count to the work
// queue size. Mirrors cloner.normalizeWorkers; duplicated rather than
// imported to keep the cmd package free of an internal cloner dep
// (this helper is 5 lines — the cost is trivial).
func normalizeBatchWorkers(requested, jobs int) int {
	if requested < 1 {
		return 1
	}
	if requested > jobs && jobs > 0 {
		return jobs
	}

	return requested
}

// indexedBatchJob carries the input-list position alongside the repo
// path so the collector can restore input order.
type indexedBatchJob struct {
	idx  int
	path string
}

// indexedBatchResult pairs a per-repo outcome with its original index.
type indexedBatchResult struct {
	idx int
	row batchRowResult
}

// processBatchReposConcurrent fans per-repo work across a bounded pool.
// Each worker pulls one repo from the job channel and writes its
// outcome to the result channel; the collector re-orders by input
// index so the CSV report rows still match the caller's repo list.
//
// Concurrency contract: processOneBatchRepo owns its own per-repo
// state (probe, version read, row construction) and shares no mutable
// data with peers, so no extra mutex is required here.
func processBatchReposConcurrent(repos []string, workers int) []batchRowResult {
	jobs := make(chan indexedBatchJob, len(repos))
	results := make(chan indexedBatchResult, len(repos))

	startBatchWorkers(workers, jobs, results)
	enqueueBatchJobs(repos, jobs)

	return collectBatchResults(repos, results)
}

// startBatchWorkers spins up the worker goroutines. Each one drains
// the job channel until it closes.
func startBatchWorkers(workers int, jobs <-chan indexedBatchJob, results chan<- indexedBatchResult) {
	for w := 0; w < workers; w++ {
		go func() {
			for j := range jobs {
				results <- indexedBatchResult{idx: j.idx, row: processOneBatchRepo(j.path)}
			}
		}()
	}
}

// enqueueBatchJobs pushes every repo onto the job channel and closes
// it so workers exit cleanly once drained.
func enqueueBatchJobs(repos []string, jobs chan<- indexedBatchJob) {
	for i, r := range repos {
		jobs <- indexedBatchJob{idx: i, path: r}
	}
	close(jobs)
}

// collectBatchResults drains the result channel and slots each row
// back into its input position.
func collectBatchResults(repos []string, results <-chan indexedBatchResult) []batchRowResult {
	out := make([]batchRowResult, len(repos))
	for i := 0; i < len(repos); i++ {
		r := <-results
		out[r.idx] = r.row
	}

	return out
}
