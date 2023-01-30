package utils

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"

	fastxz "github.com/therootcompany/xz"
)

func DownloadFile(ctx context.Context, url string, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	return Download(ctx, url, f)
}

// Download downloads a file from a given URL, and writes it to a given
// destination, which can be a file or a pipe
func Download(ctx context.Context, url string, dest io.Writer) error {

	// Request given URL

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("%s returned status code: %d", url, resp.StatusCode)
	}

	// Create a progress counter reader

	counter := &ProgressCounter{
		Ctx:  ctx,
		Op:   "Download",                 // operation
		Name: resp.Request.URL.String(),  // file name
		Size: uint64(resp.ContentLength), // file length
	}
	brdr := io.TeeReader(resp.Body, counter) // forward body reader to counter

	// Deal with response (gzip, xz, plain): reader from the counter reader (act the body reader)

	var rdr io.Reader

	switch resp.Header.Get("Content-Type") {
	case "application/x-gzip":
		rdr, err = gzip.NewReader(brdr)
		if err != nil {
			return fmt.Errorf("gzip body read: %s", err)
		}
	case "application/x-xz":
		rdr, err = fastxz.NewReader(brdr, 0)
		if err != nil {
			return fmt.Errorf("xz reader: %s", err)
		}
	default:
		rdr = brdr
	}

	_, err = io.Copy(dest, rdr) // copy to destination

	return err
}

// GetLinks returns a list of links from a given URL
func GetLinks(ctx context.Context, repoURL string) ([]string, error) {
	// Read the repo URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http request: %s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get links from %s: %s", repoURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("url %s returned %d", repoURL, resp.StatusCode)
	}

	re := regexp.MustCompile(`.*href="([^"]+)"`)

	var links []string

	// Create a progress counter reader

	counter := &ProgressCounter{
		Ctx:  ctx,
		Op:   "Download",
		Name: resp.Request.URL.String(),
		Size: uint64(resp.ContentLength),
	}

	scan := bufio.NewScanner(io.TeeReader(resp.Body, counter))

	for scan.Scan() {
		line := string(scan.Bytes())

		matches := re.FindAllStringSubmatch(line, -1)
		if matches == nil {
			continue
		}

		// Find all links in the line

		for _, m := range matches {
			res, err := url.JoinPath(repoURL, m[1])
			if err != nil {
				continue
			}
			links = append(links, res)
		}
	}

	if err := scan.Err(); err != nil {
		return nil, fmt.Errorf("error reading response: %s", err)
	}

	return links, nil
}
