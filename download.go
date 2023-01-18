package main

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

func downloadFile(ctx context.Context, url string, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	return download(ctx, url, f)
}

func download(ctx context.Context, url string, w io.Writer) error {
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
	counter := &progressCounter{Op: "Download", Name: resp.Request.URL.String(), Size: uint64(resp.ContentLength)}
	brdr := io.TeeReader(resp.Body, counter)

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

	_, err = io.Copy(w, rdr)
	return err
}

func getLinks(repourl string) ([]string, error) {
	resp, err := http.Get(repourl)
	if err != nil {
		return nil, fmt.Errorf("get links from %s: %s", repourl, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("url %s returned %d", repourl, resp.StatusCode)
	}

	var links []string
	re := regexp.MustCompile(`href="([^"]+)"`)
	counter := &progressCounter{Op: "Download", Name: resp.Request.URL.String(), Size: uint64(resp.ContentLength)}
	scan := bufio.NewScanner(io.TeeReader(resp.Body, counter))
	for scan.Scan() {
		matches := re.FindAllStringSubmatch(string(scan.Bytes()), -1)
		if matches == nil {
			continue
		}
		for _, m := range matches {
			res, err := url.JoinPath(repourl, m[1])
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
