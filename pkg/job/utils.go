package job

import (
	"context"

	"github.com/aquasecurity/btfhub/pkg/utils"
)

// GenerateBTF generates a BTF file from a vmlinux file
func GenerateBTF(ctx context.Context, vmlinux string, out string) error {
	return utils.RunCMD(ctx, "", "pahole", "--btf_gen_floats", "--skip_encoding_btf_inconsistent_proto", "--btf_gen_optimized", "--btf_encode_detached", out, vmlinux)
}
