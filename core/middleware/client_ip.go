package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// TrustedProxies lists the reverse proxies whose X-Forwarded-For entries are trusted.
type TrustedProxies []netip.Prefix

// UnmarshalText parses a comma-separated list of IPs and CIDR ranges, e.g. "10.0.0.1, 172.16.0.0/12".
func (tp *TrustedProxies) UnmarshalText(text []byte) error {
	var prefixes TrustedProxies
	for entry := range strings.SplitSeq(string(text), ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		prefix, err := parseTrustedProxy(entry)
		if err != nil {
			return fmt.Errorf("invalid trusted proxy %q: %w", entry, err)
		}
		prefixes = append(prefixes, prefix)
	}

	*tp = prefixes

	return nil
}

func parseTrustedProxy(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		prefix, err := netip.ParsePrefix(s)
		return prefix.Masked(), err
	}

	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	addr = addr.Unmap().WithZone("")

	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

// Contains reports whether addr belongs to a trusted proxy.
func (tp TrustedProxies) Contains(addr netip.Addr) bool {
	for _, prefix := range tp {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

type clientIPKey struct{}

// ClientIP resolves the client's IP address and stores it in the request context.
//
// X-Forwarded-For is only honored when the direct peer is a trusted proxy. Its entries are then
// walked from right to left, skipping trusted proxies; the first untrusted entry is the client.
// Everything further left is client-controlled and ignored. r.RemoteAddr is left untouched.
func ClientIP(trusted TrustedProxies) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), clientIPKey{}, trusted.clientIP(r))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClientIPFromContext returns the client IP resolved by ClientIP, or the zero Addr if unknown.
func ClientIPFromContext(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey{}).(netip.Addr)
	return ip
}

func (tp TrustedProxies) clientIP(r *http.Request) netip.Addr {
	client, ok := parseIP(r.RemoteAddr)
	if !ok || !tp.Contains(client) {
		return client
	}

	// Multiple X-Forwarded-For headers form a single list, in order.
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, ok := parseIP(hops[i])
		if !ok {
			// Trusted proxies only append valid IPs, so this came from the client side:
			// settle for the last trusted hop instead of guessing further.
			break
		}

		client = hop
		if !tp.Contains(hop) {
			break
		}
	}

	return client
}

// parseIP parses "ip", "ip:port" or "[ip]:port", normalized for matching against prefixes.
func parseIP(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)

	addr, err := netip.ParseAddr(s)
	if err != nil {
		addrPort, err := netip.ParseAddrPort(s)
		if err != nil {
			return netip.Addr{}, false
		}
		addr = addrPort.Addr()
	}

	return addr.Unmap().WithZone(""), true
}
