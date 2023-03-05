package job

import (
	"context"
	"log"
)

func StartWorker(ctx context.Context, jobchan <-chan Job) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobchan: // receive job (Do, Reply)
			if !ok {
				return nil
			}
			err := job.Do(ctx)
			if err != nil {
				if ch := job.Reply(); ch != nil {
					ch <- err
				} else {
					log.Printf("ERROR: %s", err)
				}
			}
		}
	}
}
