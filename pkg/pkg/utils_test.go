package pkg

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	fastxz "github.com/therootcompany/xz"
)

const filename = "test.btf"

func writeTarball(t *testing.T, out string, perm os.FileMode) (os.FileInfo, error) {
	dir := t.TempDir()
	btf := filepath.Join(dir, filename)
	err := os.WriteFile(btf, []byte{1}, perm)
	if err != nil {
		return nil, err
	}
	err = TarballBTF(context.Background(), btf, out)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(out)
	if err != nil {
		return nil, err
	}
	return stat, nil
}

func TestTarballIdempotent(t *testing.T) {
	outdir := t.TempDir()
	out1 := filepath.Join(outdir, "test1.btf.xz")
	stat1, err := writeTarball(t, out1, 0666)
	if err != nil {
		t.Fatal(err)
	}
	out2 := filepath.Join(outdir, "test2.btf.xz")
	stat2, err := writeTarball(t, out2, 0444)
	if err != nil {
		t.Fatal(err)
	}

	// check .tar.xz file attributes that matter to git
	if stat1.Size() != stat2.Size() {
		t.Errorf("file sizes do not match. file1=%d file2=%d", stat1.Size(), stat2.Size())
	}
	if stat1.Mode() != stat2.Mode() {
		t.Errorf("file modes do not match. file1=%s file2=%s", stat1.Mode(), stat2.Mode())
	}

	// check contents of file
	data1, err := os.ReadFile(out1)
	if err != nil {
		t.Fatal(err)
	}
	data2, err := os.ReadFile(out2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data1, data2) {
		t.Errorf("tarball contents are not identical")
	}

	// inspect tar contents to ensure they match our requirements
	f, err := os.Open(out1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	xr, err := fastxz.NewReader(f, 0)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(xr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.ModTime.Unix() != 0 {
			t.Errorf("BTF file timestamp is not unix epoch. time=%s unix=%d", hdr.ModTime, hdr.ModTime.Unix())
		}
		if hdr.Mode != 0444 {
			t.Errorf("BTF file mode is not 0444. mode=%o", hdr.Mode)
		}
		if hdr.Uid != 0 {
			t.Errorf("BTF file owner is not UID 0. uid=%d", hdr.Uid)
		}
		if hdr.Gid != 0 {
			t.Errorf("BTF file group is not GID 0. gid=%d", hdr.Gid)
		}
	}
}
