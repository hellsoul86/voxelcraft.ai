package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"voxelcraft.ai/internal/sim/world"
)

type JSONLZstdWriter struct {
	baseDir string
	prefix  string

	mu      sync.Mutex
	curHour string
	f       *os.File
	enc     *zstd.Encoder
	w       *bufio.Writer
}

func NewJSONLZstdWriter(baseDir, prefix string) *JSONLZstdWriter {
	return &JSONLZstdWriter{
		baseDir: baseDir,
		prefix:  prefix,
	}
}

func (w *JSONLZstdWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closeLocked()
}

func (w *JSONLZstdWriter) Write(v any) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	hour := time.Now().UTC().Format("2006-01-02-15")
	if hour != w.curHour {
		if err := w.rotateLocked(hour); err != nil {
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

func (w *JSONLZstdWriter) rotateLocked(hour string) error {
	if err := w.closeLocked(); err != nil {
		return err
	}
	dir := filepath.Dir(w.pathForHour(hour))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.pathForHour(hour), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
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
	w.curHour = hour
	return nil
}

func (w *JSONLZstdWriter) closeLocked() error {
	var err1 error
	if w.w != nil {
		_ = w.w.Flush()
	}
	if w.enc != nil {
		err1 = w.enc.Close()
		w.enc = nil
	}
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	w.w = nil
	return err1
}

func (w *JSONLZstdWriter) pathForHour(hour string) string {
	return filepath.Join(w.baseDir, fmt.Sprintf("%s-%s.jsonl.zst", w.prefix, hour))
}

// TickLogger writes one JSONL entry per tick (compressed).
type TickLogger struct{ w *JSONLZstdWriter }

func NewTickLogger(worldDir string) *TickLogger {
	return &TickLogger{w: NewJSONLZstdWriter(filepath.Join(worldDir, "events"), "events")}
}

func (l *TickLogger) WriteTick(v world.TickLogEntry) error { return l.w.Write(v) }
func (l *TickLogger) Close() error                         { return l.w.Close() }

// AuditLogger writes audit JSONL entries (compressed).
type AuditLogger struct{ w *JSONLZstdWriter }

func NewAuditLogger(worldDir string) *AuditLogger {
	return &AuditLogger{w: NewJSONLZstdWriter(filepath.Join(worldDir, "audit"), "audit")}
}

func (l *AuditLogger) WriteAudit(v world.AuditEntry) error { return l.w.Write(v) }
func (l *AuditLogger) Close() error                        { return l.w.Close() }
