package core

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustedProxies_UnmarshalText(t *testing.T) {
	var tp TrustedProxies
	require.NoError(t, tp.UnmarshalText([]byte(" 10.0.0.1, 172.16.5.4/12 ,::1,,::ffff:192.0.2.1")))

	assert.Equal(t, TrustedProxies{
		netip.MustParsePrefix("10.0.0.1/32"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("192.0.2.1/32"),
	}, tp)
}

func TestTrustedProxies_UnmarshalTextInvalid(t *testing.T) {
	for _, input := range []string{"proxy.local", "10.0.0.0/33", "10.0.0.1:80"} {
		t.Run(input, func(t *testing.T) {
			var tp TrustedProxies
			assert.ErrorContains(t, tp.UnmarshalText([]byte(input)), input)
		})
	}
}

func TestClientIP(t *testing.T) {
	var trusted TrustedProxies
	require.NoError(t, trusted.UnmarshalText([]byte("10.0.0.0/8, 2001:db8:ffff::1")))

	tests := map[string]struct {
		trusted      TrustedProxies
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
			var got netip.Addr
			h := ClientIP(tt.trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIPFromContext(r.Context())
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
	assert.False(t, ClientIPFromContext(t.Context()).IsValid())
}
