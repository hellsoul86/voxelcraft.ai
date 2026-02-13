package mcp

import (
	"bytes"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHMAC_SignAndVerify_Vector(t *testing.T) {
	secret := []byte("topsecret")
	ts := "1700000000000"
	method := "POST"
	path := "/mcp"
	body := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"list_tools\"}")

	nonce := "nonce_1"
	canon := canonicalStringV2(ts, method, path, "agent_1", nonce, body)
	got := signHMAC(secret, canon)
	want := "7695d5b4887f724294f4b74ad71d191bbf7b6ed01c87a9fb5964459543a9af47"
	if got != want {
		t.Fatalf("signature mismatch: got=%s want=%s", got, want)
	}

	req, _ := http.NewRequest(method, "http://example.invalid"+path, bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
	req.Header.Set(headerNonce, nonce)
	req.Header.Set(headerSignature, want)

	vr := verifyHMAC(req, body, secret, time.UnixMilli(1700000000000))
	if vr.HTTPStatus != 0 {
		t.Fatalf("expected ok, got status=%d msg=%s", vr.HTTPStatus, vr.Message)
	}
	if vr.SessionKey != "agent_1" {
		t.Fatalf("session key mismatch: %q", vr.SessionKey)
	}
}

func TestHMAC_Verify_Expired(t *testing.T) {
	secret := []byte("topsecret")
	ts := "1700000000000"
	body := []byte("{\"jsonrpc\":\"2.0\"}")
	nonce := "nonce_2"
	sig := signHMAC(secret, canonicalStringV2(ts, "POST", "/mcp", "agent_1", nonce, body))

	req, _ := http.NewRequest("POST", "http://example.invalid/mcp", bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
	req.Header.Set(headerNonce, nonce)
	req.Header.Set(headerSignature, sig)

	vr := verifyHMAC(req, body, secret, time.UnixMilli(1700000000000+301_000))
	if vr.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", vr.HTTPStatus)
	}
}

func TestHMAC_Verify_MissingNonce_WhenLegacyDisabled(t *testing.T) {
	origAllowLegacy, hadAllowLegacy := os.LookupEnv("VC_MCP_HMAC_ALLOW_LEGACY")
	origDeployEnv, hadDeployEnv := os.LookupEnv("DEPLOY_ENV")
	t.Cleanup(func() {
		if hadAllowLegacy {
			_ = os.Setenv("VC_MCP_HMAC_ALLOW_LEGACY", origAllowLegacy)
		} else {
			_ = os.Unsetenv("VC_MCP_HMAC_ALLOW_LEGACY")
		}
		if hadDeployEnv {
			_ = os.Setenv("DEPLOY_ENV", origDeployEnv)
		} else {
			_ = os.Unsetenv("DEPLOY_ENV")
		}
	})
	_ = os.Setenv("VC_MCP_HMAC_ALLOW_LEGACY", "false")
	_ = os.Setenv("DEPLOY_ENV", "staging")

	secret := []byte("topsecret")
	ts := "1700000000000"
	body := []byte("{\"jsonrpc\":\"2.0\"}")
	sig := signHMAC(secret, canonicalString(ts, "POST", "/mcp", body))

	req, _ := http.NewRequest("POST", "http://example.invalid/mcp", bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
	req.Header.Set(headerSignature, sig)

	vr := verifyHMAC(req, body, secret, time.UnixMilli(1700000000000))
	if vr.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", vr.HTTPStatus)
	}
	if vr.Message != "missing x-nonce" {
		t.Fatalf("expected missing x-nonce, got %q", vr.Message)
	}
}

func TestHMAC_Verify_LegacyAllowed(t *testing.T) {
	origAllowLegacy, hadAllowLegacy := os.LookupEnv("VC_MCP_HMAC_ALLOW_LEGACY")
	t.Cleanup(func() {
		if hadAllowLegacy {
			_ = os.Setenv("VC_MCP_HMAC_ALLOW_LEGACY", origAllowLegacy)
		} else {
			_ = os.Unsetenv("VC_MCP_HMAC_ALLOW_LEGACY")
		}
	})
	_ = os.Setenv("VC_MCP_HMAC_ALLOW_LEGACY", "true")

	secret := []byte("topsecret")
	ts := "1700000000000"
	body := []byte("{\"jsonrpc\":\"2.0\"}")
	sig := signHMAC(secret, canonicalString(ts, "POST", "/mcp", body))

	req, _ := http.NewRequest("POST", "http://example.invalid/mcp", bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
	req.Header.Set(headerSignature, sig)

	vr := verifyHMAC(req, body, secret, time.UnixMilli(1700000000000))
	if vr.HTTPStatus != 0 {
		t.Fatalf("expected ok, got status=%d msg=%s", vr.HTTPStatus, vr.Message)
	}
}

