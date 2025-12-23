package libTracing

import (
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

// HTTPClientSpanNameAndAttrs builds a span name and attributes for outbound HTTP calls.
// domain should be the base domain (e.g. https://service/api), and target is the full target path (path + query).
func HTTPClientSpanNameAndAttrs(apiName, domain, method, target string, timeout time.Duration, sslVerify bool) (string, map[string]string) {
	spanName := fmt.Sprintf("%s %s", apiName, method)
	if apiName == "" {
		spanName = fmt.Sprintf("HTTP %s", method)
	}

	fullURL := domain + "/" + target
	parsedURL, _ := url.Parse(fullURL)
	if parsedURL == nil {
		parsedURL, _ = url.Parse("http://" + fullURL)
	}

	attrs := map[string]string{
		"http.method": method,
		"http.url":    fullURL,
		"http.target": target,
	}

	if parsedURL != nil {
		scheme := parsedURL.Scheme
		if scheme == "" {
			scheme = "http"
		}
		host := parsedURL.Host
		if host == "" {
			host = domain
		}
		attrs["http.scheme"] = scheme
		attrs["http.host"] = host
	}

	if apiName != "" {
		attrs["api.name"] = apiName
	}
	if timeout > 0 {
		attrs["timeout"] = timeout.String()
	}
	attrs["ssl.verify"] = fmt.Sprintf("%v", sslVerify)

	return spanName, attrs
}

// MergeSpanAttrs merges extra into base, returning base (creating it if nil).
func MergeSpanAttrs(base, extra map[string]string) map[string]string {
	if base == nil && extra == nil {
		return map[string]string{}
	}
	if base == nil {
		base = map[string]string{}
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// SpanAttrsFromSlogValue flattens a slog.Value (typically from LogValue()) into span attributes.
// It flattens groups as prefix.key and stringifies values. Large values are truncated.
func SpanAttrsFromSlogValue(prefix string, v slog.Value) map[string]string {
	if prefix == "" {
		prefix = "log"
	}
	out := map[string]string{}
	addSlogValueAttrs(out, prefix, v)
	return out
}

func addSlogValueAttrs(out map[string]string, key string, v slog.Value) {
	v = v.Resolve()

	switch v.Kind() {
	case slog.KindGroup:
		for _, a := range v.Group() {
			addSlogValueAttrs(out, key+"."+a.Key, a.Value)
		}
	case slog.KindString:
		out[key] = truncateAttr(v.String())
	case slog.KindBool:
		out[key] = fmt.Sprintf("%v", v.Bool())
	case slog.KindDuration:
		out[key] = v.Duration().String()
	case slog.KindTime:
		out[key] = v.Time().Format(time.RFC3339Nano)
	case slog.KindInt64:
		out[key] = fmt.Sprintf("%d", v.Int64())
	case slog.KindUint64:
		out[key] = fmt.Sprintf("%d", v.Uint64())
	case slog.KindFloat64:
		out[key] = fmt.Sprintf("%v", v.Float64())
	case slog.KindAny:
		out[key] = truncateAttr(fmt.Sprint(v.Any()))
	default:
		out[key] = truncateAttr(v.String())
	}
}

func truncateAttr(s string) string {
	const max = 1024
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}


