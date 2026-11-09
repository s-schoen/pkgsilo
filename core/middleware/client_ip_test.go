package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"pkgsilo/core/middleware"
)

func TestClientIP(t *testing.T) {
	t.Parallel()
	trusted := []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8:ffff::1/128"),
	}

	tests := map[string]struct {
		trusted      []netip.Prefix
		remoteAddr   string
		forwardedFor []string
		wantClientIP string
	}{
		"no trusted proxies ignores header": {
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"203.0.113.7"},
			wantClientIP: "10.0.0.1",
		},
		"untrusted peer ignores header": {
			trusted:      trusted,
			remoteAddr:   "198.51.100.1:1234",
			forwardedFor: []string{"203.0.113.7"},
			wantClientIP: "198.51.100.1",
		},
		"trusted peer without header": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			wantClientIP: "10.0.0.1",
		},
		"trusted peer": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"203.0.113.7"},
			wantClientIP: "203.0.113.7",
		},
		"spoofed entries left of the client are ignored": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"6.6.6.6, 203.0.113.7"},
			wantClientIP: "203.0.113.7",
		},
		"chain of trusted proxies": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"6.6.6.6, 203.0.113.7, 10.0.0.5, 10.0.0.3"},
			wantClientIP: "203.0.113.7",
		},
		"multiple header lines": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"6.6.6.6", "203.0.113.7, 10.0.0.5"},
			wantClientIP: "203.0.113.7",
		},
		"all hops trusted": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"10.0.0.7, 10.0.0.5"},
			wantClientIP: "10.0.0.7",
		},
		"unparsable hop stops at last trusted hop": {
			trusted:      trusted,
			remoteAddr:   "10.0.0.1:1234",
			forwardedFor: []string{"203.0.113.7, garbage, 10.0.0.5"},
			wantClientIP: "10.0.0.5",
		},
		"IPv6 and ports": {
			trusted:      trusted,
			remoteAddr:   "[2001:db8:ffff::1]:443",
			forwardedFor: []string{"[2001:db8::7]:4711"},
			wantClientIP: "2001:db8::7",
		},
		"IPv4-mapped peer": {
			trusted:      trusted,
			remoteAddr:   "[::ffff:10.0.0.1]:1234",
			forwardedFor: []string{"203.0.113.7"},
			wantClientIP: "203.0.113.7",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var got netip.Addr
			h := middleware.ClientIP(tt.trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = middleware.ClientIPFromContext(r.Context())
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for _, v := range tt.forwardedFor {
				req.Header.Add("X-Forwarded-For", v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, netip.MustParseAddr(tt.wantClientIP), got)
		})
	}
}

func TestClientIPFromContext_Empty(t *testing.T) {
	t.Parallel()
	assert.False(t, middleware.ClientIPFromContext(t.Context()).IsValid())
}

func TestTrustedProxies_ContainsPeer(t *testing.T) {
	t.Parallel()
	trusted := middleware.TrustedProxies{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("2001:db8:ffff::1/128"),
	}

	tests := map[string]struct {
		remoteAddr string
		want       bool
	}{
		"trusted IPv4 peer":        {remoteAddr: "10.0.0.1:1234", want: true},
		"trusted IPv6 peer":        {remoteAddr: "[2001:db8:ffff::1]:1234", want: true},
		"IPv4-mapped trusted peer": {remoteAddr: "[::ffff:10.0.0.1]:1234", want: true},
		"untrusted peer":           {remoteAddr: "198.51.100.1:1234"},
		"unparsable peer":          {remoteAddr: "not-an-ip"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr

			assert.Equal(t, tt.want, trusted.ContainsPeer(req))
		})
	}
}
