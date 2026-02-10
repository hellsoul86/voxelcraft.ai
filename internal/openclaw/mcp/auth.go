package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	headerAgentID   = "x-agent-id"
	headerTS        = "x-ts"
	headerSignature = "x-signature"
)

func canonicalString(ts string, method string, pathname string, rawBody []byte) string {
	return ts + "\n" + strings.ToUpper(method) + "\n" + pathname + "\n" + string(rawBody)
}

func signHMAC(secret []byte, canonical string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(canonical))
	return hex.EncodeToString(h.Sum(nil))
}

type hmacVerifyResult struct {
	SessionKey string
	HTTPStatus int
	Message    string
}

func verifyHMAC(r *http.Request, rawBody []byte, secret []byte, now time.Time) hmacVerifyResult {
	agentID := strings.TrimSpace(r.Header.Get(headerAgentID))
	if agentID == "" {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-agent-id"}
	}
	tsStr := strings.TrimSpace(r.Header.Get(headerTS))
	if tsStr == "" {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-ts"}
	}
	sig := strings.TrimSpace(r.Header.Get(headerSignature))
	if sig == "" {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-signature"}
	}
	tsMS, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "bad x-ts"}
	}
	nowMS := now.UnixMilli()
	if d := nowMS - tsMS; d > 300_000 || d < -300_000 {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "x-ts outside window"}
	}

	canon := canonicalString(tsStr, r.Method, r.URL.Path, rawBody)
	exp := signHMAC(secret, canon)
	if strings.ToLower(sig) != exp {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "bad signature"}
	}
	return hmacVerifyResult{SessionKey: agentID, HTTPStatus: 0}
}

func requireLoopback(r *http.Request) error {
	host := r.RemoteAddr
	// RemoteAddr includes port; allow both "ip:port" and bare ip.
	ip := host
	if i := strings.LastIndex(host, ":"); i >= 0 {
		ip = host[:i]
	}
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return nil
	}
	return fmt.Errorf("forbidden: non-loopback client")
}

