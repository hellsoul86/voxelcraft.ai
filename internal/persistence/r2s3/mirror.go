package r2s3

import (
	"context"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	QueueDepth          int
	QueueCapacity       int
	EnqueuedTotal       uint64
	QueueSaturatedTotal uint64
	DroppedTotal        uint64
	UploadSuccessTotal  uint64
	UploadFailTotal     uint64
	LastSuccessUnix     int64
	LastErrorUnix       int64
}

type Mirror struct {
	client  *Client
	dataDir string
	prefix  string
	logger  *log.Logger

	jobs        chan string
	enqueueWait time.Duration
	wg          sync.WaitGroup

	enqueuedTotal       atomic.Uint64
	queueSaturatedTotal atomic.Uint64
	droppedTotal        atomic.Uint64
	uploadSuccessTotal  atomic.Uint64
	uploadFailTotal     atomic.Uint64
	lastSuccessUnix     atomic.Int64
	lastErrorUnix       atomic.Int64
}

func NewMirror(client *Client, dataDir, prefix string, workers, queueCapacity int, enqueueWait time.Duration, logger *log.Logger) *Mirror {
	if workers <= 0 {
		workers = 1
	}
	if queueCapacity <= 0 {
		queueCapacity = 2048
	}
	if enqueueWait <= 0 {
		enqueueWait = 25 * time.Millisecond
	}
	m := &Mirror{
		client:      client,
		dataDir:     dataDir,
		prefix:      strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/"),
		logger:      logger,
		jobs:        make(chan string, queueCapacity),
		enqueueWait: enqueueWait,
	}
	for i := 0; i < workers; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			for localPath := range m.jobs {
				m.uploadOne(localPath)
			}
		}()
	}
	return m
}

func (m *Mirror) Enqueue(localPath string) {
	if m == nil || m.client == nil {
		return
	}
	m.enqueuedTotal.Add(1)

	select {
	case m.jobs <- localPath:
		return
	default:
	}

	m.queueSaturatedTotal.Add(1)
	// Keep enqueue bounded to avoid stalling world-tick call sites, but allow
	// a short configurable wait to reduce drop risk under brief bursts.
	timer := time.NewTimer(m.enqueueWait)
	defer timer.Stop()
	select {
	case m.jobs <- localPath:
		return
	case <-timer.C:
		dropped := m.droppedTotal.Add(1)
		m.printf("r2 mirror drop local=%s reason=queue_saturated wait_ms=%d dropped_total=%d", localPath, m.enqueueWait.Milliseconds(), dropped)
	}
}

func (m *Mirror) Close() {
	if m == nil {
		return
	}
	close(m.jobs)
	m.wg.Wait()
}

func (m *Mirror) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	return Stats{
		QueueDepth:          len(m.jobs),
		QueueCapacity:       cap(m.jobs),
		EnqueuedTotal:       m.enqueuedTotal.Load(),
		QueueSaturatedTotal: m.queueSaturatedTotal.Load(),
		DroppedTotal:        m.droppedTotal.Load(),
		UploadSuccessTotal:  m.uploadSuccessTotal.Load(),
		UploadFailTotal:     m.uploadFailTotal.Load(),
		LastSuccessUnix:     m.lastSuccessUnix.Load(),
		LastErrorUnix:       m.lastErrorUnix.Load(),
	}
}

func (m *Mirror) uploadOne(localPath string) {
	key, err := m.objectKey(localPath)
	if err != nil {
		m.printf("r2 mirror skip local=%s err=%v", localPath, err)
		return
	}

	if err := m.uploadWithRetry(key, localPath); err != nil {
		m.uploadFailTotal.Add(1)
		m.lastErrorUnix.Store(time.Now().UTC().Unix())
		m.printf("r2 mirror upload failed key=%s local=%s err=%v", key, localPath, err)
		return
	}
	m.uploadSuccessTotal.Add(1)
	m.lastSuccessUnix.Store(time.Now().UTC().Unix())
	m.printf("r2 mirror uploaded key=%s local=%s", key, localPath)
}

func (m *Mirror) uploadWithRetry(key, localPath string) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := m.client.PutFile(ctx, key, localPath)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt < maxAttempts {
			backoff := time.Duration(attempt*attempt) * 200 * time.Millisecond
			time.Sleep(backoff)
		}
	}
	return lastErr
}

func (m *Mirror) objectKey(localPath string) (string, error) {
	if localPath == "" {
		return "", fmt.Errorf("empty local path")
	}
	if _, err := os.Stat(localPath); err != nil {
		return "", err
	}

	absBase, err := filepath.Abs(m.dataDir)
	if err != nil {
		return "", err
	}
	absLocal, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absBase, absLocal)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path %s is outside data dir %s", absLocal, absBase)
	}

	key := rel
	if m.prefix != "" {
		key = path.Join(m.prefix, key)
	}
	return key, nil
}

func (m *Mirror) printf(format string, args ...any) {
	if m.logger != nil {
		m.logger.Printf(format, args...)
	}
}
