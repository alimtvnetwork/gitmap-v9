package cmd

// clonetermstream.go — shared "print one RepoTermBlock right before
// a clone runs" helper used by clone, clone-now, clone-pick, and
// clone-from when the user passes `--output terminal`. Keeps every
// clone-related command byte-identical in its per-repo preview.
//
// Stream order (matches the answer locked in chat):
//
//	[block for repo i]   ← printed BEFORE we shell out to git
//	<git clone progress>
//	[block for repo i+1]
//	<git clone progress>
//	...
//
// The block goes to stdout (matches clone-next and clone-from).
// Clone progress stays on stderr per the project's stream contract.

import (
	"fmt"
	"os"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
	"github.com/alimtvnetwork/gitmap-v7/gitmap/render"
)

// CloneTermBlockInput carries the per-repo data every clone command
// already has on hand. Branch/BranchSource may be empty — the
// renderer falls back to "(unknown)" so the block shape is stable.
type CloneTermBlockInput struct {
	Index        int
	Name         string
	Branch       string
	BranchSource string
	OriginalURL  string
	TargetURL    string
	Dest         string
}

// maybePrintCloneTermBlock emits the standardized RepoTermBlock to
// stdout when output == "terminal". Any other value (including the
// empty default) is a no-op so existing CI logs and screenshots
// stay byte-identical for callers that didn't opt in.
//
// Returns nothing — write errors on stdout are surfaced by the
// caller's later Println/Printf calls. Per the zero-swallow policy
// we still log to stderr so a closed stdout doesn't silently drop
// the preview.
func maybePrintCloneTermBlock(output string, in CloneTermBlockInput) {
	if output != constants.OutputTerminal {
		return
	}
	block := render.RepoTermBlock{
		Index:        in.Index,
		Name:         in.Name,
		Branch:       in.Branch,
		BranchSource: in.BranchSource,
		OriginalURL:  in.OriginalURL,
		TargetURL:    in.TargetURL,
		CloneCommand: buildCloneCommand(in.TargetURL, in.Dest, in.Branch),
	}
	if err := render.RenderRepoTermBlock(os.Stdout, block); err != nil {
		fmt.Fprintf(os.Stderr,
			"  Warning: could not write terminal block for %s: %v\n",
			in.Name, err)
	}
}

// buildCloneCommand returns the exact `git clone` command we would
// run for the given URL+dest (+optional branch). Mirrors the shape
// produced by clone-next so users see one command shape regardless
// of which clone command they invoked.
func buildCloneCommand(url, dest, branch string) string {
	if len(branch) > 0 {
		return fmt.Sprintf("%s %s -b %s %s %s",
			constants.GitBin, constants.GitClone, branch, url, dest)
	}

	return fmt.Sprintf("%s %s %s %s",
		constants.GitBin, constants.GitClone, url, dest)
}
