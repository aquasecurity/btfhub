package job

import (
	"context"

	"github.com/aquasecurity/btfhub/pkg/utils"
)

func GenerateBTF(ctx context.Context, vmlinux string, out string) error {
	return utils.RunCMD(ctx, "", "pahole", "--btf_encode_detached", out, vmlinux)
}
