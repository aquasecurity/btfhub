package utils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	zypperMtx sync.Mutex
)

func RunZypperCMD(ctx context.Context, args ...string) (*bytes.Buffer, error) {
	zypperMtx.Lock()
	defer zypperMtx.Unlock()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	fullargs := append([]string{"zypper"}, args...)
	cmd := exec.CommandContext(ctx, "sudo", fullargs...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zypper cmd sudo %s: %s\n%s", strings.Join(fullargs, " "), err, stderr.String())
	}
	return stdout, nil
}
