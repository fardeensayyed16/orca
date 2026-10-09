// Package api contains HTTP handlers and middleware for the API.
package api

import (
	"net"
	"net/http"
	"net/netip"
)

func loopbackOnly(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		addr, err := netip.ParseAddr(host)
		if err != nil || (addr != netip.MustParseAddr("127.0.0.1") && addr != netip.IPv6Loopback()) {
			jsonError(w, "loopback access required", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}
