package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	headerAgentID   = "x-agent-id"
	headerTS        = "x-ts"
	headerSignature = "x-signature"
	headerNonce     = "x-nonce"
)

func canonicalString(ts string, method string, pathname string, rawBody []byte) string {
	return ts + "\n" + strings.ToUpper(method) + "\n" + pathname + "\n" + string(rawBody)
}

func canonicalStringV2(ts string, method string, pathname string, agentID string, nonce string, rawBody []byte) string {
	return ts + "\n" + strings.ToUpper(method) + "\n" + pathname + "\n" + strings.TrimSpace(agentID) + "\n" + strings.TrimSpace(nonce) + "\n" + string(rawBody)
}

func signHMAC(secret []byte, canonical string) string {
	h := hmac.New(sha256.New, secret)
	_, _ = h.Write([]byte(canonical))
	return hex.EncodeToString(h.Sum(nil))
}

type hmacVerifyResult struct {
	SessionKey string
	Signature  string
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
	sigRaw := strings.TrimSpace(r.Header.Get(headerSignature))
	if sigRaw == "" {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-signature"}
	}
	sig := strings.ToLower(sigRaw)
	nonce := strings.TrimSpace(r.Header.Get(headerNonce))
	allowLegacy := allowLegacyHMAC()
	if nonce == "" && !allowLegacy {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-nonce"}
	}

	tsMS, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "bad x-ts"}
	}
	nowMS := now.UnixMilli()
	if d := nowMS - tsMS; d > 300_000 || d < -300_000 {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "x-ts outside window"}
	}

	if nonce != "" {
		canonV2 := canonicalStringV2(tsStr, r.Method, r.URL.Path, agentID, nonce, rawBody)
		expV2 := signHMAC(secret, canonV2)
		if sig == expV2 {
			return hmacVerifyResult{SessionKey: agentID, Signature: sig, HTTPStatus: 0}
		}
	}

	if allowLegacy {
		canon := canonicalString(tsStr, r.Method, r.URL.Path, rawBody)
		exp := signHMAC(secret, canon)
		if sig == exp {
			return hmacVerifyResult{SessionKey: agentID, Signature: sig, HTTPStatus: 0}
		}
	}

	if nonce == "" {
		return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "missing x-nonce"}
	}
	return hmacVerifyResult{HTTPStatus: http.StatusUnauthorized, Message: "bad signature"}
}

func allowLegacyHMAC() bool {
	v := strings.TrimSpace(os.Getenv("VC_MCP_HMAC_ALLOW_LEGACY"))
	if v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEPLOY_ENV"))) {
	case "staging", "production":
		return false
	default:
		return true
	}
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

