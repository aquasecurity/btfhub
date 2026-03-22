package utils

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	fastxz "github.com/therootcompany/xz"
)

func DownloadFile(ctx context.Context, url string, file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// Large binaries (ddebs, RPMs): never use BTFHUB_HTTP_CACHE here - it would
	// duplicate the full payload on disk (.body + destination file). Metadata
	// uses Download() into buffers and GetLinks(); those still use the cache.
	return downloadNoCache(ctx, url, f)
}

// compressedLayer returns a reader that decompresses r when Content-Type indicates gzip/xz.
func compressedLayer(r io.Reader, contentType string) (io.Reader, error) {
	switch contentType {
	case "application/x-gzip":
		return gzip.NewReader(r)
	case "application/x-xz":
		return fastxz.NewReader(r, 0)
	default:
		return r, nil
	}
}

// bodyReader applies HTTP Content-Encoding (e.g. gzip) then Content-Type-based compression.
func bodyReader(r io.Reader, contentEncoding, contentType string) (io.Reader, error) {
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("gzip content-encoding: %w", err)
		}
		r = gr
	}
	return compressedLayer(r, contentType)
}

// Download downloads a file from a given URL, and writes it to a given
// destination, which can be a file or a pipe
func Download(ctx context.Context, url string, dest io.Writer) error {
	if httpCacheDir() != "" {
		return downloadHTTPCached(ctx, url, dest)
	}
	return downloadNoCache(ctx, url, dest)
}

func downloadNoCache(ctx context.Context, url string, dest io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status code: %d", url, resp.StatusCode)
	}

	counter := &ProgressCounter{
		Ctx:  ctx,
		Op:   "Download",
		Name: resp.Request.URL.String(),
		Size: uint64(resp.ContentLength),
	}
	brdr := io.TeeReader(resp.Body, counter)

	rdr, err := bodyReader(brdr, resp.Header.Get("Content-Encoding"), resp.Header.Get("Content-Type"))
	if err != nil {
		return err
	}

	_, err = io.Copy(dest, rdr)
	return err
}

// GetLinks returns a list of links from a given URL
func GetLinks(ctx context.Context, repoURL string) ([]string, error) {
	if httpCacheDir() != "" {
		return getLinksHTTPCached(ctx, repoURL)
	}
	return getLinksNoCache(ctx, repoURL)
}

func getLinksNoCache(ctx context.Context, repoURL string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http request: %s", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get links from %s: %s", repoURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("url %s returned %d", repoURL, resp.StatusCode)
	}

	return scanHTMLLinks(ctx, resp.Request.URL.String(), resp.Body, uint64(resp.ContentLength))
}

func getLinksHTTPCached(ctx context.Context, repoURL string) ([]string, error) {
	raw, meta, err := fetchRawHTTPCached(ctx, repoURL)
	if err != nil {
		return nil, err
	}
	var ce, ct string
	displayURL := repoURL
	if meta != nil {
		ce = meta.ContentEncoding
		ct = meta.ContentType
		if meta.FinalURL != "" {
			displayURL = meta.FinalURL
		}
	}
	rdr, err := bodyReader(bytes.NewReader(raw), ce, ct)
	if err != nil {
		return nil, err
	}
	decompressed, err := io.ReadAll(rdr)
	if err != nil {
		return nil, err
	}
	return scanHTMLLinks(ctx, displayURL, bytes.NewReader(decompressed), uint64(len(decompressed)))
}

func scanHTMLLinks(ctx context.Context, displayURL string, body io.Reader, size uint64) ([]string, error) {
	re := regexp.MustCompile(`.*href="([^"]+)"`)

	counter := &ProgressCounter{
		Ctx:  ctx,
		Op:   "Download",
		Name: displayURL,
		Size: size,
	}

	scan := bufio.NewScanner(io.TeeReader(body, counter))

	var links []string

	for scan.Scan() {
		line := string(scan.Bytes())

		matches := re.FindAllStringSubmatch(line, -1)
		if matches == nil {
			continue
		}

		for _, m := range matches {
			res, err := url.JoinPath(displayURL, m[1])
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
