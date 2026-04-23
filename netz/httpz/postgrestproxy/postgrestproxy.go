package postgrestproxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

// NewHandler creates a new HTTP handler that proxies requests to a PostgREST server.
//
// postgrestURL is the URL of the PostgREST server (e.g., "http://localhost:3000").
// prefix is the URL path prefix to strip from incoming requests (e.g., "/pg").
// schema is the PostgREST schema to use for the requests (e.g., "public").
// If empty, no profile headers are set.
func NewHandler(postgrestURL, prefix, schema string) (http.Handler, error) {
	target, err := url.Parse(postgrestURL)
	if err != nil {
		return nil, err
	}
	if err := validateTargetURL(target); err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{}
	proxy.Rewrite = func(r *httputil.ProxyRequest) {
		requestPath := stripPrefixPath(r.In.URL.Path, prefix)
		escapedPath := r.In.URL.EscapedPath()
		if escapedPath != "" {
			escapedPath = stripPrefixPath(escapedPath, prefix)
		}

		r.SetURL(target)
		r.SetXForwarded()
		r.Out.URL.Path = joinURLPath(target.Path, requestPath)
		if escapedPath != "" {
			r.Out.URL.RawPath = joinURLPath(target.EscapedPath(), escapedPath)
		} else {
			r.Out.URL.RawPath = ""
		}

		if schema == "" {
			return
		}

		// Remove agg server's Authorization header to avoid it being forwarded to PostgREST,
		// which causes authentication issues.
		r.Out.Header.Del("Authorization")
		if r.In.Header.Get("Prefer") == "" {
			r.Out.Header.Set("Prefer", "return=representation")
		}
		switch r.In.Method {
		case http.MethodGet, http.MethodHead:
			r.Out.Header.Set("Accept-Profile", schema)
			r.Out.Header.Del("Content-Profile")
		default:
			r.Out.Header.Set("Content-Profile", schema)
			r.Out.Header.Del("Accept-Profile")
		}
	}
	return proxy, nil
}

func validateTargetURL(target *url.URL) error {
	if target.Scheme == "" || target.Host == "" {
		return fmt.Errorf("postgrestURL must be an absolute URL with scheme and host")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return fmt.Errorf("postgrestURL scheme must be http or https")
	}
	return nil
}

// normalizePrefix normalizes the prefix by ensuring it starts with a leading slash
// and does not end with a trailing slash (unless it's just "/").
func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimSuffix(prefix, "/")
}

// normalizePath ensures the path starts with a leading slash.
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

// stripPrefixPath removes the specified prefix from the request path if it exists.
func stripPrefixPath(p, prefix string) string {
	p = normalizePath(p)
	prefix = normalizePrefix(prefix)
	if !hasPrefix(p, prefix) {
		return p
	}

	p = strings.TrimPrefix(p, prefix)
	return normalizePath(p)
}

// hasPrefix checks if requestPath has the given prefix as a path segment.
func hasPrefix(requestPath, prefix string) bool {
	if prefix == "" {
		return false
	}
	if requestPath == prefix {
		return true
	}
	return strings.HasPrefix(requestPath, prefix+"/")
}

func joinURLPath(base, requestPath string) string {
	requestPath = normalizePath(requestPath)
	if base == "" {
		return requestPath
	}
	joined := path.Join("/", base, requestPath)
	if requestPath == "/" {
		return joined + "/"
	}
	return joined
}
