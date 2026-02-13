package r2s3

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	sigV4Algorithm = "AWS4-HMAC-SHA256"
	sigV4Region    = "auto"
	sigV4Service   = "s3"
)

type Client struct {
	endpoint        string
	bucket          string
	accessKeyID     string
	secretAccessKey string
	httpClient      *http.Client
}

func New(endpoint, bucket, accessKeyID, secretAccessKey string) (*Client, error) {
	endpoint = strings.TrimSpace(endpoint)
	bucket = strings.TrimSpace(bucket)
	accessKeyID = strings.TrimSpace(accessKeyID)
	secretAccessKey = strings.TrimSpace(secretAccessKey)

	if endpoint == "" || bucket == "" || accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("endpoint/bucket/access key/secret key are required")
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
	}

	base := strings.TrimRight(u.String(), "/")
	return &Client{
		endpoint:        base,
		bucket:          bucket,
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		httpClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}, nil
}

func (c *Client) PutFile(ctx context.Context, objectKey, localPath string) error {
	objectKey = normalizeObjectKey(objectKey)
	if objectKey == "" {
		return fmt.Errorf("empty object key")
	}

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("path is directory: %s", localPath)
	}

	payloadHash, err := fileSHA256Hex(f)
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	escapedKey := escapePath(objectKey)
	canonicalURI := "/" + c.bucket + "/" + escapedKey
	requestURL := c.endpoint + canonicalURI

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, requestURL, f)
	if err != nil {
		return err
	}
	host := req.URL.Host
	req.Header.Set("Host", host)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = st.Size()

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"

	canonicalRequest := strings.Join([]string{
		http.MethodPut,
		canonicalURI,
		"",
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, sigV4Region, sigV4Service, "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		sigV4Algorithm,
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := deriveSigningKey(c.secretAccessKey, dateStamp, sigV4Region, sigV4Service)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	auth := fmt.Sprintf(
		"%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		sigV4Algorithm,
		c.accessKeyID,
		scope,
		signedHeaders,
		signature,
	)
	req.Header.Set("Authorization", auth)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	return fmt.Errorf("r2 put failed status=%d key=%s body=%s", resp.StatusCode, objectKey, strings.TrimSpace(string(body)))
}

func normalizeObjectKey(key string) string {
	key = strings.TrimSpace(strings.ReplaceAll(key, "\\", "/"))
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return ""
	}
	clean := path.Clean("/" + key)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." || strings.HasPrefix(clean, "../") {
		return ""
	}
	return clean
}

func escapePath(p string) string {
	if p == "" {
		return ""
	}
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func fileSHA256Hex(f *os.File) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func deriveSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write(data)
	return h.Sum(nil)
}
