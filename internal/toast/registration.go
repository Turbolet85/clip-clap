// Package toast — AppUserModelID registration in HKCU per Phase 3 plan
// Step 11. The key is reserved (compile-time constant) and uses the minimum
// registry.SET_VALUE permission mask per security-plan.
package toast

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// appUserModelIDParent is the HKCU subkey path beneath which per-app
// AppUserModelId entries live. Hardcoded to prevent string-interpolation
// vulnerabilities per security-plan §Input Validation (key path hardcoding).
const appUserModelIDParent = `Software\Classes\AppUserModelId\Turbolet85.ClipClap`

// DefaultAppID is the canonical clip-clap AppUserModelID used as the owner
// identifier for Windows toast notifications.
const DefaultAppID = "Turbolet85.ClipClap"

// displayNameValue is the value written under the Turbolet85.ClipClap key.
// This appears in Windows Settings → Notifications for this AppID.
const displayNameValue = "Clip Clap"

// Injectable functions for unit testing without touching the real registry.
var (
	registryOpenKey = func(k registry.Key, path string, access uint32) (registry.Key, error) {
		return registry.OpenKey(k, path, access)
	}
	registryCreateKey = func(k registry.Key, path string, access uint32) (registry.Key, bool, error) {
		return registry.CreateKey(k, path, access)
	}
	registrySetStringValue = func(k registry.Key, name, value string) error {
		return k.SetStringValue(name, value)
	}
	registryCloseKey = func(k registry.Key) error {
		return k.Close()
	}
)

// SetRegistryFunctionsForTesting swaps the registry function hooks with
// test stubs. Pass nil fns to restore defaults via ResetRegistryFunctions.
func SetRegistryFunctionsForTesting(
	openFn func(k registry.Key, path string, access uint32) (registry.Key, error),
	createFn func(k registry.Key, path string, access uint32) (registry.Key, bool, error),
	setFn func(k registry.Key, name, value string) error,
	closeFn func(k registry.Key) error,
) {
	if openFn != nil {
		registryOpenKey = openFn
	}
	if createFn != nil {
		registryCreateKey = createFn
	}
	if setFn != nil {
		registrySetStringValue = setFn
	}
	if closeFn != nil {
		registryCloseKey = closeFn
	}
}

// ResetRegistryFunctions restores production registry calls.
func ResetRegistryFunctions() {
	registryOpenKey = func(k registry.Key, path string, access uint32) (registry.Key, error) {
		return registry.OpenKey(k, path, access)
	}
	registryCreateKey = func(k registry.Key, path string, access uint32) (registry.Key, bool, error) {
		return registry.CreateKey(k, path, access)
	}
	registrySetStringValue = func(k registry.Key, name, value string) error {
		return k.SetStringValue(name, value)
	}
	registryCloseKey = func(k registry.Key) error {
		return k.Close()
	}
}

// RegisterAppUserModelID writes (idempotently) the AppUserModelID entry
// under HKCU\Software\Classes\AppUserModelId\Turbolet85.ClipClap. Returns
// nil on success, an error if registry access fails.
//
// Idempotency: the function first opens the key read-only (QUERY_VALUE).
// If it exists, returns nil WITHOUT writing (so a second invocation is a
// pure no-op that doesn't disturb the registry). If it does not exist,
// creates with SET_VALUE permission only and sets the DisplayName.
//
// Permission mask: `registry.SET_VALUE` ONLY — never `KEY_ALL_ACCESS` per
// security-plan §Win32 Resource Hygiene. The hive is hardcoded to
// HKEY_CURRENT_USER (never LOCAL_MACHINE).
func RegisterAppUserModelID(appID string) error {
	_ = appID // accepted for API compatibility; the key path is hardcoded
	// Idempotency check: open read-only.
	existing, err := registryOpenKey(registry.CURRENT_USER, appUserModelIDParent, registry.QUERY_VALUE)
	if err == nil {
		_ = registryCloseKey(existing)
		return nil // already registered — silent success
	}

	// Create with minimum write permission.
	k, _, err := registryCreateKey(registry.CURRENT_USER, appUserModelIDParent, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("CreateKey: %w", err)
	}
	defer registryCloseKey(k)

	if err := registrySetStringValue(k, "DisplayName", displayNameValue); err != nil {
		return fmt.Errorf("SetStringValue DisplayName: %w", err)
	}
	return nil
}
