package fetch

import "testing"

func TestUserAgent(t *testing.T) {
	known := map[string]bool{}
	for _, ua := range userAgents {
		known[ua] = true
	}
	seen := map[string]bool{}
	for range 64 {
		ua := userAgent()
		if !known[ua] {
			t.Fatalf("unknown %q", ua)
		}
		seen[ua] = true
	}
	if len(seen) < 2 {
		t.Fatal("stuck")
	}
}
