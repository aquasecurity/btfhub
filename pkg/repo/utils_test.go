package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aquasecurity/btfhub/pkg/job"
	"github.com/aquasecurity/btfhub/pkg/pkg"
	"github.com/aquasecurity/btfhub/pkg/utils"
)

// processWithMarker runs processPackage for a kernel in a work directory
// holding only the given file, with nobody taking jobs: a package that is
// not skipped waits for a worker until the context ends.
func processWithMarker(t *testing.T, marker string, force bool) error {
	t.Helper()

	workDir := t.TempDir()
	if marker != "" {
		if err := os.WriteFile(filepath.Join(workDir, marker), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	p := &pkg.CentOSPackage{Name: "kernel-debuginfo-1.2.3", NameOfFile: "1.2.3.el7.x86_64"}

	return processPackage(ctx, p, workDir, force, make(chan job.Job))
}

func TestProcessPackageSkips(t *testing.T) {
	tests := []struct {
		name   string
		marker string
		force  bool
		want   error
	}{
		{"existing BTF", "1.2.3.el7.x86_64.btf.tar.xz", false, nil},
		{"marked as failed", "1.2.3.el7.x86_64.failed", false, nil},
		{"marked as having BTF", "1.2.3.el7.x86_64.hasbtf", false, utils.ErrHasBTF},
		{"nothing known", "", false, context.DeadlineExceeded},
		{"marked as failed, forced", "1.2.3.el7.x86_64.failed", true, context.DeadlineExceeded},
		{"another kernel marked as failed", "1.2.4.el7.x86_64.failed", false, context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := processWithMarker(t, tt.marker, tt.force); !errors.Is(err, tt.want) {
				t.Errorf("got %v, want %v", err, tt.want)
			}
		})
	}
}
