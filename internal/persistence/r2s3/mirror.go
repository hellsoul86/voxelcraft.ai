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
	"time"
)

type Mirror struct {
	client  *Client
	dataDir string
	prefix  string
	logger  *log.Logger

	jobs chan string
	wg   sync.WaitGroup
}

func NewMirror(client *Client, dataDir, prefix string, workers int, logger *log.Logger) *Mirror {
	if workers <= 0 {
		workers = 1
	}
	m := &Mirror{
		client:  client,
		dataDir: dataDir,
		prefix:  strings.Trim(strings.ReplaceAll(prefix, "\\", "/"), "/"),
		logger:  logger,
		jobs:    make(chan string, 2048),
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
	select {
	case m.jobs <- localPath:
	default:
		// Do not block world tick paths when queue is saturated.
		go m.uploadOne(localPath)
	}
}

func (m *Mirror) Close() {
	if m == nil {
		return
	}
	close(m.jobs)
	m.wg.Wait()
}

func (m *Mirror) uploadOne(localPath string) {
	key, err := m.objectKey(localPath)
	if err != nil {
		m.printf("r2 mirror skip local=%s err=%v", localPath, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := m.client.PutFile(ctx, key, localPath); err != nil {
		m.printf("r2 mirror upload failed key=%s local=%s err=%v", key, localPath, err)
		return
	}
	m.printf("r2 mirror uploaded key=%s local=%s", key, localPath)
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
