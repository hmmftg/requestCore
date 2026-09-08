package httpsemantics

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Link represents a single RFC 8288 Web Link with a target URI and
// optional parameters.
type Link struct {
	// URI is the link target. May be relative or absolute.
	URI string

	// Rel is the link relation type (e.g. "next", "prev", "self").
	Rel string

	// Title is an optional human-readable link title.
	Title string

	// Type is an optional media type hint for the target.
	Type string

	// HrefLang is an optional language hint.
	HrefLang string

	// ExtraParams holds additional parameters not covered by the
	// fields above. Keys are parameter names, values are parameter
	// values.
	ExtraParams map[string]string
}

// Format serializes the Link as an RFC 8288 link value:
// `<uri>; rel="rel"; title="title"`.
func (l Link) Format() string {
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(l.URI)
	b.WriteString(">")

	if l.Rel != "" {
		b.WriteString(`; rel="`)
		b.WriteString(escapeLinkParam(l.Rel))
		b.WriteString(`"`)
	}
	if l.Title != "" {
		b.WriteString(`; title="`)
		b.WriteString(escapeLinkParam(l.Title))
		b.WriteString(`"`)
	}
	if l.Type != "" {
		b.WriteString(`; type="`)
		b.WriteString(escapeLinkParam(l.Type))
		b.WriteString(`"`)
	}
	if l.HrefLang != "" {
		b.WriteString(`; hreflang="`)
		b.WriteString(escapeLinkParam(l.HrefLang))
		b.WriteString(`"`)
	}
	for k, v := range l.ExtraParams {
		b.WriteString(`; `)
		b.WriteString(k)
		b.WriteString(`="`)
		b.WriteString(escapeLinkParam(v))
		b.WriteString(`"`)
	}

	return b.String()
}

// FormatLinkHeader serializes a slice of Links as a single Link header
// value, with links separated by commas.
func FormatLinkHeader(links []Link) string {
	parts := make([]string, len(links))
	for i, l := range links {
		parts[i] = l.Format()
	}
	return strings.Join(parts, ", ")
}

// PaginationLinks holds the rel types for a paginated collection.
type PaginationLinks struct {
	First string
	Prev  string
	Next  string
	Last  string
	Self  string
}

// PaginationConfig holds the parameters for building pagination links.
type PaginationConfig struct {
	// BaseURL is the request URL (absolute or relative) without query
	// parameters. May include a path.
	BaseURL string

	// Page is the current page number (1-based).
	Page int

	// PageSize is the number of items per page.
	PageSize int

	// TotalItems is the total number of items across all pages.
	// If 0, the "last" link is omitted.
	TotalItems int

	// PageParam is the query parameter name for the page number.
	// Defaults to "page".
	PageParam string

	// PageSizeParam is the query parameter name for the page size.
	// Defaults to "per_page".
	PageSizeParam string

	// Self is the URI for the "self" link relation. If empty, the
	// "self" link is omitted.
	Self string

	// ExtraParams holds additional query parameters to preserve
	// across pagination links.
	ExtraParams url.Values
}

// BuildPaginationLinks constructs RFC 8288 Link entries for a paginated
// collection. It preserves unrelated query parameters, omits
// unavailable relations (e.g. no "prev" on page 1), and safely handles
// relative or absolute request URLs.
func BuildPaginationLinks(cfg PaginationConfig) []Link {
	if cfg.PageParam == "" {
		cfg.PageParam = "page"
	}
	if cfg.PageSizeParam == "" {
		cfg.PageSizeParam = "per_page"
	}

	totalPages := 0
	if cfg.PageSize > 0 && cfg.TotalItems > 0 {
		totalPages = (cfg.TotalItems + cfg.PageSize - 1) / cfg.PageSize
	}

	var links []Link

	if cfg.Self != "" {
		links = append(links, Link{URI: cfg.Self, Rel: "self"})
	}

	// First page
	firstParams := cloneParams(cfg.ExtraParams)
	firstParams.Set(cfg.PageParam, "1")
	firstParams.Set(cfg.PageSizeParam, strconv.Itoa(cfg.PageSize))
	links = append(links, Link{URI: buildURL(cfg.BaseURL, firstParams), Rel: "first"})

	// Previous page (omit on page 1)
	if cfg.Page > 1 {
		prevParams := cloneParams(cfg.ExtraParams)
		prevParams.Set(cfg.PageParam, strconv.Itoa(cfg.Page-1))
		prevParams.Set(cfg.PageSizeParam, strconv.Itoa(cfg.PageSize))
		links = append(links, Link{URI: buildURL(cfg.BaseURL, prevParams), Rel: "prev"})
	}

	// Next page (omit if no more pages)
	if totalPages == 0 || cfg.Page < totalPages {
		nextParams := cloneParams(cfg.ExtraParams)
		nextParams.Set(cfg.PageParam, strconv.Itoa(cfg.Page+1))
		nextParams.Set(cfg.PageSizeParam, strconv.Itoa(cfg.PageSize))
		links = append(links, Link{URI: buildURL(cfg.BaseURL, nextParams), Rel: "next"})
	}

	// Last page (omit if total is unknown)
	if totalPages > 0 {
		lastParams := cloneParams(cfg.ExtraParams)
		lastParams.Set(cfg.PageParam, strconv.Itoa(totalPages))
		lastParams.Set(cfg.PageSizeParam, strconv.Itoa(cfg.PageSize))
		links = append(links, Link{URI: buildURL(cfg.BaseURL, lastParams), Rel: "last"})
	}

	return links
}

func cloneParams(src url.Values) url.Values {
	dst := make(url.Values)
	for k, v := range src {
		dst[k] = append([]string(nil), v...)
	}
	return dst
}

func buildURL(base string, params url.Values) string {
	if len(params) == 0 {
		return base
	}
	encoded := params.Encode()
	if encoded == "" {
		return base
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + encoded
}

func escapeLinkParam(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\\' || r == '"':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ParseLinkHeader parses an RFC 8288 Link header value into a slice of
// Link entries. This is a simple parser for testing and verification.
func ParseLinkHeader(s string) ([]Link, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	var links []Link
	parts := strings.Split(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		link, err := parseSingleLink(part)
		if err != nil {
			return nil, err
		}
		links = append(links, link)
	}
	return links, nil
}

func parseSingleLink(s string) (Link, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "<") {
		return Link{}, fmt.Errorf("httpsemantics: link must start with '<'")
	}
	closeIdx := strings.Index(s, ">")
	if closeIdx < 0 {
		return Link{}, fmt.Errorf("httpsemantics: link missing '>'")
	}

	link := Link{URI: s[1:closeIdx]}

	rest := strings.TrimSpace(s[closeIdx+1:])
	if rest == "" {
		return link, nil
	}

	for _, param := range strings.Split(rest, ";") {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		eqIdx := strings.Index(param, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(param[:eqIdx])
		val := strings.TrimSpace(param[eqIdx+1:])
		val = strings.Trim(val, `"`)
		switch key {
		case "rel":
			link.Rel = val
		case "title":
			link.Title = val
		case "type":
			link.Type = val
		case "hreflang":
			link.HrefLang = val
		default:
			if link.ExtraParams == nil {
				link.ExtraParams = make(map[string]string)
			}
			link.ExtraParams[key] = val
		}
	}

	return link, nil
}
