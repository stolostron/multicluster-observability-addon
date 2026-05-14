package privacy

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	errConsentRequired = errors.New("privacy: provider requires explicit user consent")
	errUnknownProvider = errors.New("privacy: unknown provider type")
)

// ValidateConsent enforces exfiltration policy for prediction providers.
func ValidateConsent(providerType string, consentGiven bool) error {
	switch providerType {
	case "builtin", "onnx":
		return nil
	case "external", "custom":
		if !consentGiven {
			return fmt.Errorf("privacy: provider %q: %w", providerType, errConsentRequired)
		}
		return nil
	default:
		return fmt.Errorf("privacy: unknown provider type %q: %w", providerType, errUnknownProvider)
	}
}

// RedactLabels returns a new map with label values replaced by SHA-256 hex digests.
func RedactLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(labels))
	for k, v := range labels {
		sum := sha256.Sum256([]byte(v))
		out[k] = hex.EncodeToString(sum[:])
	}
	return out
}
