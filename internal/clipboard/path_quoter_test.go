package clipboard

import "testing"

// TestQuote_SpaceRequiresQuotes exercises the primary decision: autoQuote=true
// + space-in-path → double-quoted; autoQuote=true + no-space → unquoted. The
// table-driven style makes it obvious that the behavior key is "presence of
// space", not "length of path" or any other input property.
func TestQuote_SpaceRequiresQuotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"path with space is wrapped in literal double-quotes", `C:\Program Files\app\a.png`, `"C:\Program Files\app\a.png"`},
		{"path without space is unwrapped", `C:\Users\foo\a.png`, `C:\Users\foo\a.png`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Quote(tc.in, true)
			if got != tc.want {
				t.Errorf("Quote(%q, true)\n  want: %q\n  got:  %q", tc.in, tc.want, got)
			}
		})
	}
}

// TestQuote_AutoQuoteDisabled — when the config flag is off, spaces in the
// path must NOT cause auto-quoting. Architecture §Clipboard Replacement
// Format explicitly lets users disable this behavior.
func TestQuote_AutoQuoteDisabled(t *testing.T) {
	in := `C:\Program Files\app\a.png`
	want := in
	got := Quote(in, false)
	if got != want {
		t.Errorf("Quote with autoQuote=false must pass through\n  want: %q\n  got:  %q", want, got)
	}
}
