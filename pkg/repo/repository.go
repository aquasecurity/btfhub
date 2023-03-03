package repo

import (
	"context"

	"github.com/aquasecurity/btfhub/pkg/job"
)

type Repository interface {
	GetKernelPackages(ctx context.Context, dir string, release string, arch string, jobchan chan<- job.Job) error
}
