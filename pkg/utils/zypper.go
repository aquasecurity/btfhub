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
	binary, args := SudoCMD("zypper", args...)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zypper cmd %s %s: %s\n%s", binary, strings.Join(args, " "), err, stderr.String())
	}
	return stdout, nil
}
