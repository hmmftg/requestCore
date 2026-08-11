package libGin

import (
	"regexp"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// Filter is a function that decides whether a request should be logged.
type Filter func(ctx *gin.Context) bool

// Accept returns the given filter unchanged.
func Accept(filter Filter) Filter { return filter }

// Ignore returns a filter that negates the given filter.
func Ignore(filter Filter) Filter { return func(ctx *gin.Context) bool { return !filter(ctx) } }

// AcceptMethod returns a filter that accepts requests matching any of the given HTTP methods.
func AcceptMethod(methods ...string) Filter {
	return func(c *gin.Context) bool {
		reqMethod := strings.ToLower(c.Request.Method)

		for _, method := range methods {
			if strings.ToLower(method) == reqMethod {
				return true
			}
		}

		return false
	}
}

// IgnoreMethod returns a filter that rejects requests matching any of the given methods.
func IgnoreMethod(methods ...string) Filter {
	return func(c *gin.Context) bool {
		reqMethod := strings.ToLower(c.Request.Method)

		for _, method := range methods {
			if strings.ToLower(method) == reqMethod {
				return false
			}
		}

		return true
	}
}

// AcceptStatus returns a filter that accepts responses matching any of the given statuses.
func AcceptStatus(statuses ...int) Filter {
	return func(c *gin.Context) bool {
		return slices.Contains(statuses, c.Writer.Status())
	}
}

// IgnoreStatus returns a filter that rejects responses matching any of the given statuses.
func IgnoreStatus(statuses ...int) Filter {
	return func(c *gin.Context) bool {
		return !slices.Contains(statuses, c.Writer.Status())
	}
}

// AcceptStatusGreaterThan returns a filter that accepts responses with status greater than the given value.
func AcceptStatusGreaterThan(status int) Filter {
	return func(c *gin.Context) bool {
		return c.Writer.Status() > status
	}
}

// AcceptStatusGreaterThanOrEqual returns a filter that accepts responses with status greater than or equal to the given value.
func AcceptStatusGreaterThanOrEqual(status int) Filter {
	return func(c *gin.Context) bool {
		return c.Writer.Status() >= status
	}
}

// AcceptStatusLessThan returns a filter that accepts responses with status less than the given value.
func AcceptStatusLessThan(status int) Filter {
	return func(c *gin.Context) bool {
		return c.Writer.Status() < status
	}
}

// AcceptStatusLessThanOrEqual returns a filter that accepts responses with status less than or equal to the given value.
func AcceptStatusLessThanOrEqual(status int) Filter {
	return func(c *gin.Context) bool {
		return c.Writer.Status() <= status
	}
}

// IgnoreStatusGreaterThan returns a filter that rejects responses with status greater than the given value.
func IgnoreStatusGreaterThan(status int) Filter {
	return AcceptStatusLessThanOrEqual(status)
}

// IgnoreStatusGreaterThanOrEqual returns a filter that rejects responses with status greater than or equal to the given value.
func IgnoreStatusGreaterThanOrEqual(status int) Filter {
	return AcceptStatusLessThan(status)
}

// IgnoreStatusLessThan returns a filter that rejects responses with status less than the given value.
func IgnoreStatusLessThan(status int) Filter {
	return AcceptStatusGreaterThanOrEqual(status)
}

// IgnoreStatusLessThanOrEqual returns a filter that rejects responses with status less than or equal to the given value.
func IgnoreStatusLessThanOrEqual(status int) Filter {
	return AcceptStatusGreaterThan(status)
}

// AcceptPath returns a filter that accepts requests whose path matches any of the given URLs.
func AcceptPath(urls ...string) Filter {
	return func(c *gin.Context) bool {
		return slices.Contains(urls, c.Request.URL.Path)
	}
}

// IgnorePath returns a filter that rejects requests whose path matches any of the given URLs.
func IgnorePath(urls ...string) Filter {
	return func(c *gin.Context) bool {
		return !slices.Contains(urls, c.Request.URL.Path)
	}
}

// AcceptPathContains returns a filter that accepts requests whose path contains any of the given parts.
func AcceptPathContains(parts ...string) Filter {
	return func(c *gin.Context) bool {
		for _, part := range parts {
			if strings.Contains(c.Request.URL.Path, part) {
				return true
			}
		}

		return false
	}
}

// IgnorePathContains returns a filter that rejects requests whose path contains any of the given parts.
func IgnorePathContains(parts ...string) Filter {
	return func(c *gin.Context) bool {
		for _, part := range parts {
			if strings.Contains(c.Request.URL.Path, part) {
				return false
			}
		}

		return true
	}
}

// AcceptPathPrefix returns a filter that accepts requests whose path starts with any of the given prefixes.
func AcceptPathPrefix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				return true
			}
		}

		return false
	}
}

// IgnorePathPrefix returns a filter that rejects requests whose path starts with any of the given prefixes.
func IgnorePathPrefix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				return false
			}
		}

		return true
	}
}

// AcceptPathSuffix returns a filter that accepts requests whose path ends with any of the given suffixes.
func AcceptPathSuffix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				return true
			}
		}

		return false
	}
}

// IgnorePathSuffix returns a filter that rejects requests whose path ends with any of the given suffixes.
func IgnorePathSuffix(suffixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, suffix := range suffixs {
			if strings.HasSuffix(c.Request.URL.Path, suffix) {
				return false
			}
		}

		return true
	}
}

// AcceptPathMatch returns a filter that accepts requests whose path matches any of the given regexps.
func AcceptPathMatch(regs ...regexp.Regexp) Filter {
	return func(c *gin.Context) bool {
		for _, reg := range regs {
			if reg.Match([]byte(c.Request.URL.Path)) {
				return true
			}
		}

		return false
	}
}

// IgnorePathMatch returns a filter that rejects requests whose path matches any of the given regexps.
func IgnorePathMatch(regs ...regexp.Regexp) Filter {
	return func(c *gin.Context) bool {
		for _, reg := range regs {
			if reg.Match([]byte(c.Request.URL.Path)) {
				return false
			}
		}

		return true
	}
}

// AcceptHost returns a filter that accepts requests whose host matches any of the given hosts.
func AcceptHost(hosts ...string) Filter {
	return func(c *gin.Context) bool {
		return slices.Contains(hosts, c.Request.URL.Host)
	}
}

// IgnoreHost returns a filter that rejects requests whose host matches any of the given hosts.
func IgnoreHost(hosts ...string) Filter {
	return func(c *gin.Context) bool {
		return !slices.Contains(hosts, c.Request.URL.Host)
	}
}

// AcceptHostContains returns a filter that accepts requests whose host contains any of the given parts.
func AcceptHostContains(parts ...string) Filter {
	return func(c *gin.Context) bool {
		for _, part := range parts {
			if strings.Contains(c.Request.URL.Host, part) {
				return true
			}
		}

		return false
	}
}

// IgnoreHostContains returns a filter that rejects requests whose host contains any of the given parts.
func IgnoreHostContains(parts ...string) Filter {
	return func(c *gin.Context) bool {
		for _, part := range parts {
			if strings.Contains(c.Request.URL.Host, part) {
				return false
			}
		}

		return true
	}
}

// AcceptHostPrefix returns a filter that accepts requests whose host starts with any of the given prefixes.
func AcceptHostPrefix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Host, prefix) {
				return true
			}
		}

		return false
	}
}

// IgnoreHostPrefix returns a filter that rejects requests whose host starts with any of the given prefixes.
func IgnoreHostPrefix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Host, prefix) {
				return false
			}
		}

		return true
	}
}

// AcceptHostSuffix returns a filter that accepts requests whose host ends with any of the given suffixes.
func AcceptHostSuffix(prefixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(c.Request.URL.Host, prefix) {
				return true
			}
		}

		return false
	}
}

// IgnoreHostSuffix returns a filter that rejects requests whose host ends with any of the given suffixes.
func IgnoreHostSuffix(suffixs ...string) Filter {
	return func(c *gin.Context) bool {
		for _, suffix := range suffixs {
			if strings.HasSuffix(c.Request.URL.Host, suffix) {
				return false
			}
		}

		return true
	}
}

// AcceptHostMatch returns a filter that accepts requests whose host matches any of the given regexps.
func AcceptHostMatch(regs ...regexp.Regexp) Filter {
	return func(c *gin.Context) bool {
		for _, reg := range regs {
			if reg.Match([]byte(c.Request.URL.Host)) {
				return true
			}
		}

		return false
	}
}

// IgnoreHostMatch returns a filter that rejects requests whose host matches any of the given regexps.
func IgnoreHostMatch(regs ...regexp.Regexp) Filter {
	return func(c *gin.Context) bool {
		for _, reg := range regs {
			if reg.Match([]byte(c.Request.URL.Host)) {
				return false
			}
		}

		return true
	}
}
