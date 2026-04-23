package postgrest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripPrefixPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{
			name:   "ExactMatch",
			path:   "/pg",
			prefix: "/pg",
			want:   "/",
		},
		{
			name:   "Subpath",
			path:   "/pg/items",
			prefix: "/pg",
			want:   "/items",
		},
		{
			name:   "TrailingSlashPrefix",
			path:   "/pg/items",
			prefix: "/pg/",
			want:   "/items",
		},
		{
			name:   "NonSegmentPrefix",
			path:   "/pgx/items",
			prefix: "/pg",
			want:   "/pgx/items",
		},
		{
			name:   "EmptyPath",
			path:   "",
			prefix: "/pg",
			want:   "/",
		},
		{
			name:   "EmptyPrefix",
			path:   "/items",
			prefix: "",
			want:   "/items",
		},
		{
			name:   "RootPrefix",
			path:   "/items",
			prefix: "/",
			want:   "/items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripPrefixPath(tt.path, tt.prefix)
			assert.Equal(t, tt.want, got, "stripPrefixPath(%q, %q)", tt.path, tt.prefix)
		})
	}
}

func TestValidateTargetURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "relative url", url: "/v1"},
		{name: "missing scheme", url: "localhost:3000"},
		{name: "unsupported scheme", url: "ftp://localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHandler(tt.url, "/pg", "sage")
			require.Error(t, err)
		})
	}
}

func TestProxyBehavior(t *testing.T) {
	t.Run("StripsStrictPrefix", func(t *testing.T) {
		var gotHost, gotPath string
		upstream := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				gotHost = r.Host
				gotPath = r.URL.Path
				w.WriteHeader(http.StatusNoContent)
			}))
		defer upstream.Close()

		h, err := NewHandler(upstream.URL+"/base", "/pg", "sage")
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/pgx/items", nil)
		req.Host = "client.example"
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)

		u, err := url.Parse(upstream.URL)
		require.NoError(t, err)
		assert.Equal(t, u.Host, gotHost)
		assert.Equal(t, "/base/pgx/items", gotPath)
	})

	t.Run("ProfileHeaders", func(t *testing.T) {
		var gotAcceptProfile, gotContentProfile string
		upstream := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				gotAcceptProfile = r.Header.Get("Accept-Profile")
				gotContentProfile = r.Header.Get("Content-Profile")
				w.WriteHeader(http.StatusNoContent)
			}))
		defer upstream.Close()

		h, err := NewHandler(upstream.URL, "/pg", "sage")
		require.NoError(t, err)

		recGet := httptest.NewRecorder()
		reqGet := httptest.NewRequest(http.MethodGet, "http://example.com/pg/items", nil)
		h.ServeHTTP(recGet, reqGet)
		assert.Equal(t, "sage", gotAcceptProfile)
		assert.Empty(t, gotContentProfile)

		recPost := httptest.NewRecorder()
		reqPost := httptest.NewRequest(http.MethodPost, "http://example.com/pg/items", nil)
		h.ServeHTTP(recPost, reqPost)
		assert.Empty(t, gotAcceptProfile)
		assert.Equal(t, "sage", gotContentProfile)

		recOptions := httptest.NewRecorder()
		reqOptions := httptest.NewRequest(http.MethodOptions, "http://example.com/pg/items", nil)
		reqOptions.Header.Set("Accept-Profile", "incoming")
		reqOptions.Header.Set("Content-Profile", "incoming")
		h.ServeHTTP(recOptions, reqOptions)
		assert.Empty(t, gotAcceptProfile)
		assert.Equal(t, "sage", gotContentProfile)
	})

	t.Run("EscapedPath", func(t *testing.T) {
		var gotPath, gotRawPath string
		upstream := httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotRawPath = r.URL.RawPath
				w.WriteHeader(http.StatusNoContent)
			}))
		defer upstream.Close()

		h, err := NewHandler(upstream.URL, "/pg", "")
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/pg/a%2Fb", nil)
		req.URL.RawPath = "/pg/a%2Fb"
		h.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Equal(t, "/a/b", gotPath)
		assert.Equal(t, "/a%2Fb", gotRawPath)
	})
}
