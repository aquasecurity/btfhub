package main

import (
	"context"
	"log"
)

type Job interface {
	Do(context.Context) error
	Reply() chan<- interface{}
}

func StartWorker(ctx context.Context, jobchan <-chan Job) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobchan:
			if !ok {
				return nil
			}
			err := job.Do(ctx)
			if err != nil {
				log.Printf("ERROR: %s", err)
				if ch := job.Reply(); ch != nil {
					ch <- err
				}
			}
		}
	}
}
