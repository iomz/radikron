//go:build darwin

package radikron

import "testing"

func TestPathContains(t *testing.T) {
	for _, tt := range []struct {
		path string
		dir  string
		want bool
	}{
		{"/bin:/usr/local/bin", "/usr/local/bin", true},
		{"/bin:/usr/local/bin-extra", "/usr/local/bin", false},
		{"", "/opt/homebrew/bin", false},
	} {
		if got := pathContains(tt.path, tt.dir); got != tt.want {
			t.Errorf("pathContains(%q, %q) = %t, want %t", tt.path, tt.dir, got, tt.want)
		}
	}
}

func TestWithHomebrewPaths(t *testing.T) {
	got := withHomebrewPaths("/custom/bin")
	want := "/custom/bin:/opt/homebrew/bin:/usr/local/bin"
	if got != want {
		t.Fatalf("withHomebrewPaths() = %q, want %q", got, want)
	}
	got = withHomebrewPaths("/usr/local/bin:/custom/bin")
	want = "/usr/local/bin:/custom/bin:/opt/homebrew/bin"
	if got != want {
		t.Fatalf("withHomebrewPaths(existing) = %q, want %q", got, want)
	}
}
