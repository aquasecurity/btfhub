package main

import "testing"

func TestVersionOrdering(t *testing.T) {
	v1 := newKernelVersion("3.10.0-957")
	v2 := newKernelVersion("3.10.0-957.el7")
	if v1.Less(v2) {
		t.Fatalf("%s must not be less than %s", v1, v2)
	}

	v3 := newKernelVersion("3.10.0-956")
	if !v3.Less(v2) {
		t.Fatalf("%s must be less than %s", v3, v2)
	}

	v4 := newKernelVersion("3.10.0-957.100")
	if !v1.Less(v4) {
		t.Fatalf("%s must be less than %s", v1, v4)
	}
}
