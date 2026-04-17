package clipboard

import (
	"fmt"
	"strings"
)

// Quote wraps path in double quotes when autoQuote is true AND the path
// contains a space; otherwise returns the path unchanged. The architecture
// (§Clipboard Replacement Format) requires auto-quoting of paths-with-spaces
// so pasted paths survive shell tokenization in Windows Terminal / WSL / SSH.
//
// Pure function — no clipboard I/O. Phase 3's clipboard.go will call Quote
// before invoking Win32 SetClipboardData with the result.
func Quote(path string, autoQuote bool) string {
	if !autoQuote || !strings.Contains(path, " ") {
		return path
	}
	// Wrap with literal double-quotes WITHOUT escaping backslashes. `%q`
	// would escape `\` to `\\` which breaks Windows paths when pasted into
	// a shell (the shell would interpret `\\` as an escape, not a separator).
	return fmt.Sprintf("\"%s\"", path)
}
