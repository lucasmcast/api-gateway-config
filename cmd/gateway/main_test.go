package main

import "testing"

func TestRemovedCommands(t *testing.T) {
	for _, command := range []string{"build", "check"} {
		if err := run([]string{command}); err == nil {
			t.Fatalf("accepted removed command %s", command)
		}
	}
}
