// Package middleware provides the HTTP middleware shared by all features.
package middleware

import (
	"context"
	"net/http"
	"net/netip"
	"slices"
	"strings"

	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// TrustedProxies lists the reverse proxies whose X-Forwarded-For entries are trusted. It must not
// hold IPv4-mapped IPv6 prefixes, since addresses are matched unmapped.
type TrustedProxies []netip.Prefix

// Contains reports whether addr belongs to a trusted proxy.
func (tp TrustedProxies) Contains(addr netip.Addr) bool {
	for _, prefix := range tp {
		if prefix.Contains(addr) {
			return true
		}
	}

	return false
}

// ContainsPeer reports whether the request's direct peer, its RemoteAddr, is a trusted proxy.
func (tp TrustedProxies) ContainsPeer(r *http.Request) bool {
	peer, ok := parseIP(r.RemoteAddr)

	return ok && tp.Contains(peer)
}

type clientIPKey struct{}

// ClientIP resolves the client's IP address and stores it in the request context. It also sets it as
// client.address on the span in the request context, overwriting what otelhttp took from
// X-Forwarded-For whatever the peer, or the empty string if the peer's address can't be parsed.
//
// X-Forwarded-For is only honored when the direct peer is a trusted proxy. Its entries are then
// walked from right to left, skipping trusted proxies; the first untrusted entry is the client.
// Everything further left is client-controlled and ignored. r.RemoteAddr is left untouched.
// Peers are matched as unmapped IPs, so trusted must not hold IPv4-mapped IPv6 prefixes.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	tp := TrustedProxies(trusted)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := tp.clientIP(r)
			address := ""
			if ip.IsValid() {
				address = ip.String()
			}
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.ClientAddress(address))

			ctx := context.WithValue(r.Context(), clientIPKey{}, ip)
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
	for _, entry := range slices.Backward(hops) {
		hop, ok := parseIP(entry)
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
