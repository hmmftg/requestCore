package nextadapter

import (
	"log/slog"
)

// safeLogValue returns the value to log for a typed response in the
// mandatory <operation>-req success AddLog entry. If the response
// implements slog.LogValuer, its LogValue projection is used so
// response-type owners can mask sensitive fields. A panic in LogValue
// is recovered and reported as a masking failure; the raw response is
// never logged as a fallback.
//
// This mirrors the safety behavior of handlers.logValueForAddLog so the
// new bridge preserves the enterprise AddLog masking contract.
func safeLogValue(resp any) any {
	if lv, ok := resp.(slog.LogValuer); ok {
		var val slog.Value
		panicked := true
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("nextadapter: LogValue panic masked",
						slog.Any("panic", r))
				}
			}()
			val = lv.LogValue()
			panicked = false
		}()
		if panicked {
			return slog.StringValue("<masked: logvalue-panic>")
		}
		return val
	}
	return resp
}
