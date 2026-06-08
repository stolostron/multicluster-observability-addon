package virtualization

import commonSdk "github.com/perses/perses/go-sdk/common"

// strPtr returns a pointer to a copy of s, allowing string constants to be
// used where *string is required (e.g. commonSdk.Format.Unit).
func strPtr(s string) *string { return &s }

// Package-level unit strings. All panel files in this package reference these
// rather than declaring their own locals, so the unit catalog stays in one place.
var (
	opsPerSecUnit      = "ops/sec"
	decBytesPerSecUnit = string(commonSdk.BytesDecPerSecondsUnit) // "decbytes/sec"
	dateTimeLocalUnit  = "datetime-local"
	relativeTimeUnit   = "relative-time"
)
