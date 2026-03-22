package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// BTFHUB_HTTP_CACHE, when set to a directory path, enables a simple on-disk cache
// for utils.Download (streaming to non-file writers) and utils.GetLinks: responses
// are keyed by URL (SHA-256), and conditional requests (If-None-Match /
// If-Modified-Since) avoid re-downloading unchanged metadata (APT Packages indices,
// repo directory listings, etc.). utils.DownloadFile does not use this cache so
// large package downloads are not duplicated on disk.
const httpCacheEnvVar = "BTFHUB_HTTP_CACHE"

// Client without automatic gzip/deflate so ETag/Last-Modified match the on-disk body we persist.
var httpCacheClient = &http.Client{
	Transport: &http.Transport{
		DisableCompression: true,
	},
}

func httpCacheDir() string {
	return strings.TrimSpace(os.Getenv(httpCacheEnvVar))
}

func httpCacheKey(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:])
}

// One mutex per cache key so concurrent workers do not clobber the same .body.part / rename.
var httpCacheKeyLocks sync.Map // string (hex key) -> *sync.Mutex

func lockHTTPCacheKey(key string) func() {
	v, _ := httpCacheKeyLocks.LoadOrStore(key, new(sync.Mutex))
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

type httpCacheMeta struct {
	ETag            string `json:"etag,omitempty"`
	LastModified    string `json:"last_modified,omitempty"`
	ContentType     string `json:"content_type,omitempty"`
	ContentEncoding string `json:"content_encoding,omitempty"`
	// FinalURL is the request URL after redirects (used by GetLinks for relative hrefs).
	FinalURL string `json:"final_url,omitempty"`
}

func readHTTPMeta(path string) (*httpCacheMeta, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m httpCacheMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func writeHTTPMeta(path string, m *httpCacheMeta) error {
	if m == nil {
		return errors.New("nil meta")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func applyConditionalHeaders(req *http.Request, meta *httpCacheMeta) {
	if meta == nil {
		return
	}
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}
}

func metaFromResponse(resp *http.Response) *httpCacheMeta {
	return &httpCacheMeta{
		ETag:            resp.Header.Get("ETag"),
		LastModified:    resp.Header.Get("Last-Modified"),
		ContentType:     resp.Header.Get("Content-Type"),
		ContentEncoding: resp.Header.Get("Content-Encoding"),
		FinalURL:        resp.Request.URL.String(),
	}
}

// downloadHTTPCached streams url into dest, persisting raw bytes under httpCacheDir when possible.
func downloadHTTPCached(ctx context.Context, urlStr string, dest io.Writer) error {
	dir := httpCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("http cache mkdir: %w", err)
	}
	key := httpCacheKey(urlStr)
	unlock := lockHTTPCacheKey(key)
	defer unlock()

	bodyPath := filepath.Join(dir, key+".body")
	metaPath := filepath.Join(dir, key+".meta")

	meta, _ := readHTTPMeta(metaPath)
	if meta != nil {
		if _, err := os.Stat(bodyPath); err != nil {
			meta = nil // incomplete cache entry
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return err
	}
	applyConditionalHeaders(req, meta)

	resp, err := httpCacheClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && meta != nil {
		f, err := os.Open(bodyPath)
		if err != nil {
			return fmt.Errorf("http cache 304 but missing body %s: %w", bodyPath, err)
		}
		defer f.Close()
		st, _ := f.Stat()
		var sz uint64
		if st != nil {
			sz = uint64(st.Size())
		}
		counter := &ProgressCounter{
			Ctx:  ctx,
			Op:   "Download",
			Name: urlStr,
			Size: sz,
		}
		brdr := io.TeeReader(f, counter)
		rdr, err := bodyReader(brdr, meta.ContentEncoding, meta.ContentType)
		if err != nil {
			return err
		}
		_, err = io.Copy(dest, rdr)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned status code: %d", urlStr, resp.StatusCode)
	}

	newMeta := metaFromResponse(resp)
	counter := &ProgressCounter{
		Ctx:  ctx,
		Op:   "Download",
		Name: resp.Request.URL.String(),
		Size: uint64(resp.ContentLength),
	}

	tmpBody := bodyPath + ".part"
	out, err := os.Create(tmpBody)
	if err != nil {
		return fmt.Errorf("http cache create part: %w", err)
	}
	success := false
	defer func() {
		out.Close()
		if !success {
			_ = os.Remove(tmpBody)
		}
	}()

	mw := io.MultiWriter(counter, out)
	brdr := io.TeeReader(resp.Body, mw)
	rdr, err := bodyReader(brdr, newMeta.ContentEncoding, newMeta.ContentType)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dest, rdr); err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpBody, bodyPath); err != nil {
		return fmt.Errorf("http cache finalize body: %w", err)
	}
	if err := writeHTTPMeta(metaPath, newMeta); err != nil {
		return fmt.Errorf("http cache write meta: %w", err)
	}
	success = true
	return nil
}

// fetchRawHTTPCached returns the full response body bytes (as sent on the wire, still compressed if any).
func fetchRawHTTPCached(ctx context.Context, urlStr string) ([]byte, *httpCacheMeta, error) {
	dir := httpCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("http cache mkdir: %w", err)
	}
	key := httpCacheKey(urlStr)
	unlock := lockHTTPCacheKey(key)
	defer unlock()

	bodyPath := filepath.Join(dir, key+".body")
	metaPath := filepath.Join(dir, key+".meta")

	meta, _ := readHTTPMeta(metaPath)
	if meta != nil {
		if _, err := os.Stat(bodyPath); err != nil {
			meta = nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, nil, err
	}
	applyConditionalHeaders(req, meta)

	resp, err := httpCacheClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified && meta != nil {
		b, err := os.ReadFile(bodyPath)
		if err != nil {
			return nil, nil, fmt.Errorf("http cache 304 read body: %w", err)
		}
		return b, meta, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("url %s returned %d", urlStr, resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	newMeta := metaFromResponse(resp)
	tmpBody := bodyPath + ".part"
	if err := os.WriteFile(tmpBody, raw, 0o644); err != nil {
		return nil, nil, err
	}
	if err := os.Rename(tmpBody, bodyPath); err != nil {
		return nil, nil, err
	}
	if err := writeHTTPMeta(metaPath, newMeta); err != nil {
		return nil, nil, err
	}
	return raw, newMeta, nil
}
