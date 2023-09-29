package utils

import (
	"bytes"
	"context"
	"debug/elf"
	"fmt"
	"os/exec"
	"strings"
)

// HasBTFSection checks if the given ELF file has a .BTF section.
func HasBTFSection(name string) (bool, error) {
	ef, err := elf.Open(name)
	if err != nil {
		return false, fmt.Errorf("elf open: %s", err)
	}

	return ef.Section(".BTF") != nil, nil
}

func RunCMD(ctx context.Context, cwd string, binary string, args ...string) error {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = cwd
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %s\n%s\n%s", binary, strings.Join(args, " "),
			err, stdout.String(), stderr.String())
	}

	return nil
}

func SudoCMD(binary string, args ...string) (string, []string) {
	_, err := exec.LookPath("sudo")
	if err == nil {
		return "sudo", append([]string{binary}, args...)
	}
	return binary, args
}
