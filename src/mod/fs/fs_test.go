package fs

import "testing"

func TestWithin(t *testing.T) {
	if !Within("/media", "/media/Shows") {
		t.Fatal("child must be within root")
	}
	if Within("/media", "/other") {
		t.Fatal("other must not be within root")
	}
}
