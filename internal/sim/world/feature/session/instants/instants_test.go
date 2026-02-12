package instants

import "testing"

func TestValidateSayInput(t *testing.T) {
	ok, code, _ := ValidateSayInput(" ")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected empty text rejection")
	}
	ok, code, _ = ValidateSayInput("hello")
	if !ok || code != "" {
		t.Fatalf("expected say input accepted")
	}
}

func TestValidateWhisperInput(t *testing.T) {
	ok, code, _ := ValidateWhisperInput("", "hello")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected missing target rejection")
	}
	ok, code, _ = ValidateWhisperInput("A2", "hello")
	if !ok || code != "" {
		t.Fatalf("expected whisper input accepted")
	}
}

func TestValidateSaveMemoryInput(t *testing.T) {
	ok, code, _ := ValidateSaveMemoryInput("")
	if ok || code != "E_BAD_REQUEST" {
		t.Fatalf("expected missing key rejection")
	}
	ok, code, _ = ValidateSaveMemoryInput("agent/friend")
	if !ok || code != "" {
		t.Fatalf("expected memory key accepted")
	}
}
