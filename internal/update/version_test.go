package update

import "testing"

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"0.10.0", "v0.11.0", true},
		{"v0.11.0", "0.11.0", false},
		{"0.11.0", "0.10.9", false},
		{"0.9.0", "0.10.0", true},
		{"1.0.0-rc.1", "1.0.0", true},
		{"1.0.0", "1.0.0-rc.1", false},
		{"0.10.0-next", "0.10.0", true},
		{"0.10.0+build.5", "0.10.0", false},
	}
	for _, tt := range tests {
		got, err := IsNewer(tt.current, tt.latest)
		if err != nil {
			t.Errorf("IsNewer(%q, %q) error: %v", tt.current, tt.latest, err)
			continue
		}
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestIsNewerRejectsInvalidVersions(t *testing.T) {
	for _, v := range []string{"(devel)", "", "1.2", "1.2.x", "1.-2.3"} {
		if _, err := IsNewer(v, "1.0.0"); err == nil {
			t.Errorf("IsNewer(%q, ...) accepted an invalid version", v)
		}
	}
}
