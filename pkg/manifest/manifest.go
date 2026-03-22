// Package manifest supports recording kernel packages that need work during
// preflight (JSONL) and filtering the generate pass to only those entries.
package manifest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

type ctxWriterKey struct{}
type ctxTemplateKey struct{}
type ctxFilterKey struct{}

// Template identifies one (distro, release, arch) plane for manifest lines.
type Template struct {
	Distro  string
	Release string
	Arch    string
}

// Entry is one JSONL record (Ubuntu: name_of_file is BTFFilename / kernel stem).
type Entry struct {
	Distro     string `json:"distro"`
	Release    string `json:"release"`
	Arch       string `json:"arch"`
	NameOfFile string `json:"name_of_file"`
}

// Writer appends JSONL lines (thread-safe). Count is the number of Append calls
// in this process (used for preflight exit status).
type Writer struct {
	mu    sync.Mutex
	f     *os.File
	enc   *json.Encoder
	count atomic.Int64
}

// NewAppender opens path for append (and create). Caller must Close.
func NewAppender(path string) (*Writer, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	w := &Writer{f: f, enc: json.NewEncoder(f)}
	return w, nil
}

func (w *Writer) Append(t Template, nameOfFile string) error {
	if nameOfFile == "" {
		return errors.New("manifest: empty name_of_file")
	}
	e := Entry{Distro: t.Distro, Release: t.Release, Arch: t.Arch, NameOfFile: nameOfFile}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.enc.Encode(e); err != nil {
		return err
	}
	w.count.Add(1)
	return nil
}

func (w *Writer) Count() int64 {
	return w.count.Load()
}

func (w *Writer) Close() error {
	if w == nil || w.f == nil {
		return nil
	}
	return w.f.Close()
}

// WithWriter attaches a manifest writer to ctx (used with preflight).
func WithWriter(ctx context.Context, w *Writer) context.Context {
	return context.WithValue(ctx, ctxWriterKey{}, w)
}

// WriterFromContext returns the writer, or nil.
func WriterFromContext(ctx context.Context) *Writer {
	v, _ := ctx.Value(ctxWriterKey{}).(*Writer)
	return v
}

// WithTemplate attaches distro/release/arch for manifest lines.
func WithTemplate(ctx context.Context, t Template) context.Context {
	return context.WithValue(ctx, ctxTemplateKey{}, t)
}

// TemplateFromContext returns the template and whether it was set.
func TemplateFromContext(ctx context.Context) (Template, bool) {
	t, ok := ctx.Value(ctxTemplateKey{}).(Template)
	return t, ok
}

// Filter matches (distro, release, arch, name_of_file) keys from a manifest file.
type Filter struct {
	keys map[string]struct{}
}

func makeKey(distro, release, arch, nameOfFile string) string {
	var b strings.Builder
	b.Grow(len(distro) + len(release) + len(arch) + len(nameOfFile) + 8)
	b.WriteString(distro)
	b.WriteByte(0)
	b.WriteString(release)
	b.WriteByte(0)
	b.WriteString(arch)
	b.WriteByte(0)
	b.WriteString(nameOfFile)
	return b.String()
}

// LoadFilter reads JSONL manifest from path. An empty file yields a filter
// that matches nothing. The file must exist.
func LoadFilter(path string) (*Filter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	keys := make(map[string]struct{})
	sc := bufio.NewScanner(f)
	// Long lines for safety
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("manifest line %d: %w", lineNo, err)
		}
		if e.Distro == "" || e.Release == "" || e.Arch == "" || e.NameOfFile == "" {
			return nil, fmt.Errorf("manifest line %d: missing field", lineNo)
		}
		keys[makeKey(e.Distro, e.Release, e.Arch, e.NameOfFile)] = struct{}{}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return &Filter{keys: keys}, nil
}

func (f *Filter) Match(distro, release, arch, nameOfFile string) bool {
	if f == nil || f.keys == nil {
		return true
	}
	_, ok := f.keys[makeKey(distro, release, arch, nameOfFile)]
	return ok
}

// WithFilter attaches a filter (may be nil = no filtering).
func WithFilter(ctx context.Context, f *Filter) context.Context {
	return context.WithValue(ctx, ctxFilterKey{}, f)
}

// FilterFromContext returns the filter, or nil if unset (treat as no filter).
func FilterFromContext(ctx context.Context) *Filter {
	v, _ := ctx.Value(ctxFilterKey{}).(*Filter)
	return v
}
