package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriterAppendAndLoadFilter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "m.jsonl")

	w, err := NewAppender(p)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := Template{Distro: "ubuntu", Release: "focal", Arch: "x86_64"}
	if err := w.Append(tmpl, "5.4.0-1-generic"); err != nil {
		t.Fatal(err)
	}
	if err := w.Append(tmpl, "5.4.0-2-generic"); err != nil {
		t.Fatal(err)
	}
	if w.Count() != 2 {
		t.Fatalf("count = %d", w.Count())
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	f, err := LoadFilter(p)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Match("ubuntu", "focal", "x86_64", "5.4.0-1-generic") {
		t.Fatal("expected match")
	}
	if f.Match("ubuntu", "focal", "x86_64", "5.4.0-9-generic") {
		t.Fatal("unexpected match")
	}
	if f.Match("ubuntu", "bionic", "x86_64", "5.4.0-1-generic") {
		t.Fatal("unexpected match on release")
	}
}

func TestLoadFilterMissing(t *testing.T) {
	_, err := LoadFilter(filepath.Join(t.TempDir(), "nope.jsonl"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected IsNotExist, got %v", err)
	}
}
