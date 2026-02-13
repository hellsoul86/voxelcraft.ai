package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"voxelcraft.ai/internal/sim/world"
)

type LoggerOptions struct {
	// RotateLayout controls file rotation cadence using Go time formatting.
	// Default: hourly ("2006-01-02-15").
	RotateLayout string

	// OnClose is called when a rotated/current segment is closed.
	// The callback is invoked asynchronously.
	OnClose func(path string)
}

type JSONLZstdWriter struct {
	baseDir string
	prefix  string

	mu sync.Mutex

	curWindow string
	curPath   string

	f   *os.File
	enc *zstd.Encoder
	w   *bufio.Writer

	rotateLayout string
	onClose      func(path string)
}

func NewJSONLZstdWriter(baseDir, prefix string, options ...LoggerOptions) *JSONLZstdWriter {
	opts := LoggerOptions{}
	if len(options) > 0 {
		opts = options[0]
	}
	layout := strings.TrimSpace(opts.RotateLayout)
	if layout == "" {
		layout = "2006-01-02-15"
	}
	return &JSONLZstdWriter{
		baseDir:       baseDir,
		prefix:        prefix,
		rotateLayout:  layout,
		onClose:       opts.OnClose,
	}
}

func (w *JSONLZstdWriter) Close() error {
	w.mu.Lock()
	closedPath, err := w.closeCurrentLocked()
	w.mu.Unlock()

	if closedPath != "" {
		w.notifyClosed(closedPath)
	}
	return err
}

func (w *JSONLZstdWriter) Write(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	window := time.Now().UTC().Format(w.rotateLayout)
	if window != w.curWindow {
		if err := w.rotateLocked(window); err != nil {
			return err
		}
	}

	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := w.w.Write(b); err != nil {
		return err
	}
	if err := w.w.WriteByte('\n'); err != nil {
		return err
	}
	return w.w.Flush()
}

func (w *JSONLZstdWriter) rotateLocked(window string) error {
	closedPath, err := w.closeCurrentLocked()
	if err != nil {
		return err
	}
	if closedPath != "" {
		w.notifyClosed(closedPath)
	}

	path := w.pathForWindow(window)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	enc, err := zstd.NewWriter(f, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		_ = f.Close()
		return err
	}

	w.f = f
	w.enc = enc
	w.w = bufio.NewWriterSize(enc, 128*1024)
	w.curWindow = window
	w.curPath = path
	return nil
}

func (w *JSONLZstdWriter) closeCurrentLocked() (closedPath string, err error) {
	if w.w == nil && w.enc == nil && w.f == nil {
		return "", nil
	}
	closedPath = w.curPath
	if w.w != nil {
		_ = w.w.Flush()
	}
	if w.enc != nil {
		err = w.enc.Close()
		w.enc = nil
	}
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	w.w = nil
	w.curPath = ""
	w.curWindow = ""
	return closedPath, err
}

func (w *JSONLZstdWriter) notifyClosed(path string) {
	if path == "" || w.onClose == nil {
		return
	}
	cb := w.onClose
	go cb(path)
}

func (w *JSONLZstdWriter) pathForWindow(window string) string {
	return filepath.Join(w.baseDir, fmt.Sprintf("%s-%s.jsonl.zst", w.prefix, window))
}

// TickLogger writes one JSONL entry per tick (compressed).
type TickLogger struct{ w *JSONLZstdWriter }

func NewTickLogger(worldDir string) *TickLogger {
	return NewTickLoggerWithOptions(worldDir, LoggerOptions{})
}

func NewTickLoggerWithOptions(worldDir string, opts LoggerOptions) *TickLogger {
	return &TickLogger{w: NewJSONLZstdWriter(filepath.Join(worldDir, "events"), "events", opts)}
}

func (l *TickLogger) WriteTick(v world.TickLogEntry) error { return l.w.Write(v) }
func (l *TickLogger) Close() error                         { return l.w.Close() }

// AuditLogger writes audit JSONL entries (compressed).
type AuditLogger struct{ w *JSONLZstdWriter }

func NewAuditLogger(worldDir string) *AuditLogger {
	return NewAuditLoggerWithOptions(worldDir, LoggerOptions{})
}

func NewAuditLoggerWithOptions(worldDir string, opts LoggerOptions) *AuditLogger {
	return &AuditLogger{w: NewJSONLZstdWriter(filepath.Join(worldDir, "audit"), "audit", opts)}
}

func (l *AuditLogger) WriteAudit(v world.AuditEntry) error { return l.w.Write(v) }
func (l *AuditLogger) Close() error                        { return l.w.Close() }
