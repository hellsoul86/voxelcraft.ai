package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"voxelcraft.ai/internal/persistence/r2s3"
)

type r2MirrorRuntime struct {
	enabled      bool
	rotateLayout string
	mirror       *r2s3.Mirror
}

func buildR2MirrorRuntime(dataDir string, logger *log.Logger) (*r2MirrorRuntime, error) {
	enabled := envBool("VC_R2_MIRROR", false)
	if !enabled {
		return &r2MirrorRuntime{enabled: false}, nil
	}

	endpoint := strings.TrimSpace(os.Getenv("VC_R2_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("VC_R2_BUCKET"))
	accessKeyID := strings.TrimSpace(os.Getenv("VC_R2_ACCESS_KEY_ID"))
	secretAccessKey := strings.TrimSpace(os.Getenv("VC_R2_SECRET_ACCESS_KEY"))
	prefix := strings.TrimSpace(os.Getenv("VC_R2_PREFIX"))

	if endpoint == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("VC_R2_MIRROR=true but VC_R2_ENDPOINT/VC_R2_BUCKET/VC_R2_ACCESS_KEY_ID/VC_R2_SECRET_ACCESS_KEY are not fully set")
	}

	client, err := r2s3.New(endpoint, bucket, accessKeyID, secretAccessKey)
	if err != nil {
		return nil, err
	}

	workers := envInt("VC_R2_UPLOAD_WORKERS", 2)
	mirror := r2s3.NewMirror(client, dataDir, prefix, workers, logger)

	return &r2MirrorRuntime{
		enabled:      true,
		rotateLayout: "2006-01-02-15-04", // 1-minute segments to lower RPO.
		mirror:       mirror,
	}, nil
}

func (r *r2MirrorRuntime) Close() {
	if r == nil || r.mirror == nil {
		return
	}
	r.mirror.Close()
}

func (r *r2MirrorRuntime) Enqueue(localPath string) {
	if r == nil || !r.enabled || r.mirror == nil {
		return
	}
	r.mirror.Enqueue(localPath)
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}
