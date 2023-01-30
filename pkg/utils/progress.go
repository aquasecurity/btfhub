package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"
)

type ProgressCounter struct {
	Ctx  context.Context
	Op   string
	Size uint64
	Name string

	written uint64

	lastReport time.Time
}

// Write implements the io.Writer interface and is used to count the number of
// bytes written to the underlying writer.
func (wc *ProgressCounter) Write(p []byte) (int, error) {
	if wc.Ctx != nil && wc.Ctx.Err() != nil {
		return 0, wc.Ctx.Err()
	}

	n := len(p)
	wc.written += uint64(n)

	if wc.written == wc.Size || time.Since(wc.lastReport) > 10*time.Second {
		wc.printProgress()
	}

	return n, nil
}

func (wc *ProgressCounter) printProgress() {

	// Clear the line by using a character return to go back to the start and
	// remove the remaining characters by filling it with spaces
	//
	// fmt.Printf("\r%s", strings.Repeat(" ", 35))
	// fmt.Printf("%s\n", time.Since(wc.lastReport))

	// Return again and print current status of download

	pct := uint64((float64(wc.written) / float64(wc.Size)) * 100)

	fmt.Printf("%sing %s: %s / %s - %d%% complete\n",
		wc.Op,
		wc.Name,
		humanize.Bytes(wc.written),
		humanize.Bytes(wc.Size),
		pct,
	)

	wc.lastReport = time.Now()
}
