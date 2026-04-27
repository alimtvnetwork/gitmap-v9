//go:build windows

package startup

// Registry-backend Remove + List for Windows. Pairs with
// winregistry_windows.go (which has Add). Split for the per-file
// budget and so each operation is independently scannable.
//
// Ownership model: the Run key holds ONLY the direct autostart
// value (`gitmap-<name> = "<exec>"`); the tracking subkey under
// HKCU\Software\Gitmap\StartupRegistry\<name> is the SOLE marker.
// Remove deletes the Run value only when the tracking subkey
// confirms gitmap ownership; List enumerates the tracking scope
// first, then verifies each entry has a live Run value.

import (
	"fmt"
	"strings"

	"github.com/alimtvnetwork/gitmap-v7/gitmap/constants"
	"golang.org/x/sys/windows/registry"
)

// removeWindowsRegistry deletes a managed Run-key value plus the
// tracking subkey. Returns RemoveNoOp if the value does not exist,
// RemoveRefused if it exists but the tracking subkey under
// HKCU\Software\Gitmap is missing. Mirrors the Linux RemoveStatus
// contract one-for-one.
func removeWindowsRegistry(clean string, opts RemoveOptions) (RemoveResult, error) {
	valueName := constants.StartupWinValuePrefix + clean
	exists, managed, err := classifyRunValue(valueName, clean)
	if err != nil {

		return RemoveResult{}, err
	}
	if !exists {

		return RemoveResult{Status: RemoveNoOp, DryRun: opts.DryRun}, nil
	}
	if !managed {

		return RemoveResult{Status: RemoveRefused, Path: runValuePath(valueName), DryRun: opts.DryRun}, nil
	}
	if opts.DryRun {

		return RemoveResult{Status: RemoveDeleted, Path: runValuePath(valueName), DryRun: true}, nil
	}
	if err := deleteRunValue(valueName); err != nil {

		return RemoveResult{}, err
	}
	if err := deleteTrackingSubkey(constants.RegGitmapRegistrySub, clean); err != nil {

		return RemoveResult{}, err
	}

	return RemoveResult{Status: RemoveDeleted, Path: runValuePath(valueName)}, nil
}

// deleteRunValue removes the single autostart value. Missing value
// is not an error — a previous partial Remove (crash after value
// delete, before subkey delete) would re-enter here with the value
// already gone, and the subkey-delete pass below should still run.
func deleteRunValue(valueName string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		constants.RegRunKeyPath, registry.SET_VALUE)
	if err != nil {

		return fmt.Errorf(constants.ErrStartupRegistryOpen, constants.RegRunKeyPath, err)
	}
	defer k.Close()

	if err := k.DeleteValue(valueName); err != nil && err != registry.ErrNotExist {

		return fmt.Errorf(constants.ErrStartupRegistryWrite, valueName, err)
	}

	return nil
}

// deleteTrackingSubkey removes HKCU\<parent>\<name>. Missing key is
// not an error — Add could have failed mid-way and left only the
// Run value, in which case the missing tracking subkey is expected.
func deleteTrackingSubkey(parent, name string) error {
	full := parent + `\` + name
	if err := registry.DeleteKey(registry.CURRENT_USER, full); err != nil &&
		err != registry.ErrNotExist {

		return fmt.Errorf(constants.ErrStartupRegistryWrite, full, err)
	}

	return nil
}

// listWindowsRegistry enumerates Run-key values whose name starts
// with the gitmap- prefix AND whose tracking subkey under
// HKCU\Software\Gitmap\StartupRegistry confirms gitmap ownership.
// No sibling-marker filter needed — the Run key now contains only
// real autostart values, never a `.gitmap-managed` companion.
// Returns nil (NOT an error) when the Run key itself does not
// exist — fresh accounts shouldn't see a scary error from
// `gitmap startup-list`.
func listWindowsRegistry() ([]Entry, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		constants.RegRunKeyPath, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		if err == registry.ErrNotExist {

			return nil, nil
		}

		return nil, fmt.Errorf(constants.ErrStartupRegistryOpen, constants.RegRunKeyPath, err)
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {

		return nil, fmt.Errorf(constants.ErrStartupRegistryRead, constants.RegRunKeyPath, err)
	}

	return collectRegistryManaged(k, names), nil
}

// collectRegistryManaged is the per-entry filter. Skips:
//   - values without the gitmap- name prefix
//   - values whose tracking subkey under HKCU\Software\Gitmap
//     is missing (third-party values that happen to share our
//     prefix — refuse to claim them)
func collectRegistryManaged(k registry.Key, names []string) []Entry {
	var out []Entry
	for _, name := range names {
		if !strings.HasPrefix(name, constants.StartupWinValuePrefix) {
			continue
		}
		entry, ok := readManagedRegistryValue(k, name)
		if !ok {
			continue
		}
		out = append(out, entry)
	}

	return out
}

// readManagedRegistryValue is the per-name re-check. Reads the
// command value and verifies the tracking subkey exists; returns
// ok=false if either is missing.
func readManagedRegistryValue(k registry.Key, valueName string) (Entry, bool) {
	exec, _, err := k.GetStringValue(valueName)
	if err != nil {

		return Entry{}, false
	}
	clean := strings.TrimPrefix(valueName, constants.StartupWinValuePrefix)
	if !trackingSubkeyExists(constants.RegGitmapRegistrySub, clean) {

		return Entry{}, false
	}

	return Entry{
		Name: valueName,
		Path: runValuePath(valueName),
		Exec: exec,
	}, true
}
