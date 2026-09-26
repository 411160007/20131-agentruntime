// Package auditlog persists validated Events as JSONL to a single local
// file. It is the only persistence exit of the collector.
//
// Privacy architecture: this package imports only the file system —
// no socket, dial, or HTTP types exist anywhere in the collector's code
// path, so audit data cannot leave the machine it was produced on.
package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"20131.com/agentruntime/internal/schema"
)

// Writer appends newline-delimited JSON records to one audit file.
type Writer struct {
	mu      sync.Mutex
	f       *os.File
	path    string
	written int64
	count   int
}

// Open creates (or appends to) the audit file at path.
// The file is created with 0600: audit contents are sensitive by design.
func Open(path string) (*Writer, error) {
	if path == "" {
		return nil, fmt.Errorf("auditlog: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("auditlog: mkdir %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("auditlog: open %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("auditlog: stat %s: %w", path, err)
	}
	return &Writer{f: f, path: path, written: st.Size()}, nil
}

// WriteEvent validates e, marshals it as one JSONL line, and appends it.
// Invalid events are rejected without touching the file.
func (w *Writer) WriteEvent(e *schema.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("auditlog: marshal %s: %w", e.ID, err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.f.Write(append(line, '\n'))
	if err != nil {
		return fmt.Errorf("auditlog: write %s: %w", w.path, err)
	}
	w.written += int64(n)
	w.count++
	return nil
}

// Count returns the number of events appended by this writer instance.
func (w *Writer) Count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.count
}

// Bytes returns the current on-disk size of the audit file.
// Rotation and retention policy arrive with the discovery collector;
// size is tracked here so the later policy has a cheap source of truth.
func (w *Writer) Bytes() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.written
}

// Path returns the audit file location.
func (w *Writer) Path() string { return w.path }

// Close flushes and closes the underlying file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Sync()
	if cerr := w.f.Close(); err == nil {
		err = cerr
	}
	w.f = nil
	return err
}
