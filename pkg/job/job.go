package job

import "context"

type Job interface {
	Do(context.Context) error
	Reply() chan<- interface{}
}
