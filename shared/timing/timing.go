// Package timing provides encode/decode helpers for propagating timestamps across process boundaries.
package timing

import (
	"strconv"
	"time"
)

// HeaderTimestamp is the key used for both HTTP header and gRPC metadata.
// Lowercase is used because gRPC metadata keys are case-sensitive and conventionally lowercase,
// while HTTP header lookup is case-insensitive regardless.
const HeaderTimestamp = "x-timestamp"

// EncodeTimestamp converts a time.Time into a Unix nanosecond string for wire transport.
func EncodeTimestamp(t time.Time) string {
	return strconv.FormatInt(t.UnixNano(), 10)
}

// DecodeTimestamp parses a Unix nanosecond string back into time.Time.
func DecodeTimestamp(s string) (time.Time, error) {
	nano, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, nano), nil
}
