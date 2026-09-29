package cli

import "testing"

func TestCheckOS(t *testing.T) {
	for goos, ok := range map[string]bool{"darwin": true, "linux": true, "windows": false, "freebsd": false} {
		if err := checkOS(goos); (err == nil) != ok {
			t.Errorf("checkOS(%q) = %v, want ok=%v", goos, err, ok)
		}
	}
}
