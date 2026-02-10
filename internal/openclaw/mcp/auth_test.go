package mcp

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

func TestHMAC_SignAndVerify_Vector(t *testing.T) {
	secret := []byte("topsecret")
	ts := "1700000000000"
	method := "POST"
	path := "/mcp"
	body := []byte("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"list_tools\"}")

	canon := canonicalString(ts, method, path, body)
	got := signHMAC(secret, canon)
	want := "8d8937fdcea524a9301a74e8e2e4b3ee64ea5ae993f29219d57d0cd3d276613b"
	if got != want {
		t.Fatalf("signature mismatch: got=%s want=%s", got, want)
	}

	req, _ := http.NewRequest(method, "http://example.invalid"+path, bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
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
	sig := signHMAC(secret, canonicalString(ts, "POST", "/mcp", body))

	req, _ := http.NewRequest("POST", "http://example.invalid/mcp", bytes.NewReader(body))
	req.Header.Set(headerAgentID, "agent_1")
	req.Header.Set(headerTS, ts)
	req.Header.Set(headerSignature, sig)

	vr := verifyHMAC(req, body, secret, time.UnixMilli(1700000000000+301_000))
	if vr.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", vr.HTTPStatus)
	}
}

