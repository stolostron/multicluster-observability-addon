package privacy

import (
	"strings"
	"testing"
)

func TestValidateConsent_BuiltinOK(t *testing.T) {
	if err := ValidateConsent("builtin", false); err != nil {
		t.Fatalf("builtin without consent: %v", err)
	}
}

func TestValidateConsent_OnnxOK(t *testing.T) {
	if err := ValidateConsent("onnx", false); err != nil {
		t.Fatalf("onnx without consent: %v", err)
	}
}

func TestValidateConsent_ExternalNeedsConsent(t *testing.T) {
	if err := ValidateConsent("external", false); err == nil {
		t.Fatal("external without consent: expected error")
	}
	if err := ValidateConsent("external", true); err != nil {
		t.Fatalf("external with consent: %v", err)
	}
}

func TestValidateConsent_CustomNeedsConsent(t *testing.T) {
	if err := ValidateConsent("custom", false); err == nil {
		t.Fatal("custom without consent: expected error")
	}
	if err := ValidateConsent("custom", true); err != nil {
		t.Fatalf("custom with consent: %v", err)
	}
}

func TestRedactLabels_ValuesHashed(t *testing.T) {
	in := map[string]string{"ns": "default"}
	out := RedactLabels(in)
	v, ok := out["ns"]
	if !ok {
		t.Fatal(`expected key "ns" in output`)
	}
	if v == "default" {
		t.Fatal("expected hashed value, got plaintext default")
	}
	if len(v) != 64 || !isHex(v) {
		t.Fatalf("expected 64-char hex digest, got %q", v)
	}
}

func TestRedactLabels_Consistency(t *testing.T) {
	in := map[string]string{"ns": "default"}
	a := RedactLabels(in)
	b := RedactLabels(in)
	if a["ns"] != b["ns"] {
		t.Fatalf("same input should yield same hash: %q vs %q", a["ns"], b["ns"])
	}
}

func TestRedactLabels_NilInput(t *testing.T) {
	out := RedactLabels(nil)
	if out == nil {
		t.Fatal("expected non-nil empty map")
	}
	if len(out) != 0 {
		t.Fatalf("expected empty map, got len=%d %#v", len(out), out)
	}
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
