package records

import "time"

// Dates, as apps store them.

// appleEpoch is when Apple's dates count from.
var appleEpoch = time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)

// appleDate reads a date in seconds, or nanoseconds, since 2001.
func appleDate(v any) string {
	var t time.Time
	switch v := v.(type) {
	case int64:
		if v == 0 {
			return ""
		}
		if v > 1e12 || v < -1e12 {
			t = appleEpoch.Add(time.Duration(v))
		} else {
			t = appleEpoch.Add(time.Duration(v) * time.Second)
		}
	case float64:
		if v == 0 {
			return ""
		}
		t = appleEpoch.Add(time.Duration(v * float64(time.Second)))
	default:
		return format(v)
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// unixMillis reads a date in milliseconds since 1970, as Android
// stores them.
func unixMillis(v any) string {
	ms, ok := v.(int64)
	if !ok {
		return format(v)
	}
	if ms == 0 {
		return ""
	}
	if ms < 1e11 {
		ms *= 1000 // seconds, in older tables
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}
