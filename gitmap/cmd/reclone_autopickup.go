package cmd

// Auto-discovery of a scan artifact for `gitmap reclone` when the
// user invoked the command with no <file> positional argument.
//
// Convention: `gitmap scan` always writes its artifacts under
// ./.gitmap/output/ relative to the scanned root. We treat the
// process CWD as that root and probe the canonical filenames in a
// fixed priority order (JSON first because it is the richest /
// least-lossy representation, CSV as a fallback for environments
// that only kept the spreadsheet view).
//
// We deliberately do NOT walk upward or scan sibling directories:
// silent path-magic across a tree would be surprising and would make
// "which manifest fed this reclone?" un-answerable from the command
// line alone. The MsgCloneNowAutoPickup stderr line documents the
// chosen path so the run is fully reproducible.

import (
	"os"
	"path/filepath"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
)

// autoPickupRecloneManifest returns the first existing scan-artifact
// path under ./.gitmap/output/ (CWD-relative) and ok=true, or "",
// false if neither candidate file is present. Only regular files
// count -- a directory at the candidate path is treated as a miss
// rather than silently used.
func autoPickupRecloneManifest() (string, bool) {
	base := filepath.Join(constants.GitMapDir, constants.OutputDirName)
	candidates := []string{
		filepath.Join(base, constants.DefaultJSONFile),
		filepath.Join(base, constants.DefaultCSVFile),
	}
	for _, path := range candidates {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {

			return path, true
		}
	}

	return "", false
}
