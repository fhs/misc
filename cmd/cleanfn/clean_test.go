package main

import (
	"testing"
)

var names = []struct {
	before, after string
}{
	{"[ABC]_hello_world_123_[720p][12345678].mkv",
		"ABC.hello_world_123.720p.12345678.mkv"},
	{"The\"File\"", "TheFile"},
}

func TestClean(t *testing.T) {
	for _, n := range names {
		if c := clean(n.before); c != n.after {
			t.Errorf("clean(%s) = %s; expected %s\n", n.before, c, n.after)
		}
	}
}
