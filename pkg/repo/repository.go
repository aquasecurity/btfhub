package repo

import (
	"context"

	"github.com/aquasecurity/btfhub/pkg/job"
)

type Repository interface {
	GetKernelPackages(
		ctx context.Context,
		workDir string,
		release string,
		arch string,
		jobChan chan<- job.Job,
	) error
}
