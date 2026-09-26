// Rotation extends the D1 append-only JSONL writer with segment policies:
// size-based rotation and UTC day-cut rotation (both parameterized), plus
// optional history trimming.
//
// The guarantee inherited from Writer holds unchanged: an event that
// fails Validate() is rejected before any rotation check runs, so an
// invalid event never moves bytes and never triggers a segment swap.
package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"20131.com/agentruntime/internal/schema"
)

// Rotation configures segment policy. Zero values mean "no rotation on
// that axis".
type Rotation struct {
	// MaxBytes closes and renames the current segment once the file
	// reaches this size after a write (0 = unlimited).
	MaxBytes int64
	// Daily rotates when an event's UTC date differs from the date the
	// current segment was opened/rotated to (day-cut boundary).
	Daily bool
	// KeepHistory caps rotated segments retained on disk; 0 = keep all
	// (the honest-audit default: rotation never silently deletes).
	KeepHistory int
}

// segmentTimeLayout makes rotated names sortable and reversible.
const segmentTimeLayout = "20060102T150405Z"

// Log is a rotating audit log. Events are appended through the embedded
// rotation-aware path; Path() stays the stable live file name while
// rotated history lives at path+".<UTC timestamp>".
type Log struct {
	mu      sync.Mutex
	path    string
	rot     Rotation
	w       *Writer
	curDate string // YYYY-MM-DD of current segment (empty until first write)
	count   int
}

// OpenLog opens (or appends to) the live segment at path. rot may be nil
// for plain append-only behavior identical to Open().
func OpenLog(path string, rot *Rotation) (*Log, error) {
	w, err := Open(path)
	if err != nil {
		return nil, err
	}
	l := &Log{path: path, w: w}
	if rot != nil {
		l.rot = *rot
	}
	return l, nil
}

// WriteEvent validates first (zero-byte rejection invariant preserved),
// then applies day-cut before the write and size checks after it.
func (l *Log) WriteEvent(e *schema.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if _, err := json.Marshal(e); err != nil { // cheap pre-check like Writer does
		return fmt.Errorf("auditlog: marshal %s: %w", e.ID, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.rot.Daily {
		d := e.TS.UTC().Format("2006-01-02")
		if l.curDate != "" && d != l.curDate {
			if err := l.rotateLocked(); err != nil {
				return err
			}
		}
		// The live segment belongs to the date of its first event; a
		// wall-clock stamp here would re-rotate on the next write.
		l.curDate = d
	}

	if err := l.w.WriteEvent(e); err != nil {
		return err
	}
	l.count++
	if l.rot.MaxBytes > 0 && l.w.Bytes() >= l.rot.MaxBytes {
		if err := l.rotateLocked(); err != nil {
			return err
		}
	}
	return nil
}

// rotateLocked closes the current segment, renames it with a UTC
// timestamp, reopens a fresh live file at the stable path, and prunes
// history when KeepHistory is set. Caller holds l.mu.
func (l *Log) rotateLocked() error {
	if err := l.w.Close(); err != nil {
		return fmt.Errorf("auditlog: close before rotate: %w", err)
	}
	newName, err := l.renameLocked()
	if err != nil {
		return err
	}
	w, err := Open(l.path)
	if err != nil {
		return fmt.Errorf("auditlog: reopen after rotate: %w", err)
	}
	l.w = w
	if l.rot.KeepHistory > 0 {
		l.pruneLocked()
	}
	_ = newName
	return nil
}

func (l *Log) renameLocked() (string, error) {
	stamp := time.Now().UTC().Format(segmentTimeLayout)
	target := l.path + "." + stamp
	for i := 1; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("auditlog: stat rotate target: %w", err)
		}
		target = fmt.Sprintf("%s.%s.%d", l.path, stamp, i)
	}
	if err := os.Rename(l.path, target); err != nil {
		return "", fmt.Errorf("auditlog: rename %s -> %s: %w", l.path, target, err)
	}
	return target, nil
}

var segmentSuffix = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}Z(\.[0-9]+)?$`)

// segments lists rotated segment paths (oldest first).
func (l *Log) segments() []string {
	dir := filepath.Dir(l.path)
	base := filepath.Base(l.path) + "."
	matches, _ := filepath.Glob(filepath.Join(dir, base+"*"))
	var out []string
	for _, m := range matches {
		if segmentSuffix.MatchString(strings.TrimPrefix(m, filepath.Join(dir, base))) {
			out = append(out, m)
		}
	}
	sort.Strings(out) // UTC stamps sort chronologically
	return out
}

// pruneLocked deletes the oldest segments beyond KeepHistory. Only files
// with a strict rotation-stamp suffix are ever removed — unrelated files
// sharing the base prefix are left alone.
func (l *Log) pruneLocked() {
	segs := l.segments()
	for len(segs) > l.rot.KeepHistory {
		_ = os.Remove(segs[0])
		segs = segs[1:]
	}
}

// Count returns events written through this Log since open.
func (l *Log) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// Bytes returns the current live segment size.
func (l *Log) Bytes() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Bytes()
}

// Path returns the stable live segment path.
func (l *Log) Path() string { return l.path }

// Segments returns the rotated history files, oldest first (exported for
// retention visibility).
func (l *Log) Segments() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.segments()
}

// Close flushes and closes the live segment; rotated history is untouched.
func (l *Log) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Close()
}
