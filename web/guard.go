package web

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/mesa-black/stellar"
)

// The server connects to hosts named by strangers. Left unchecked, that is a
// way to make it probe the network it runs in. Two guards apply: the name
// must look like a public DNS name, and the address actually dialled must be
// a public one. The second check runs at connect time, on the resolved
// address, so a name that resolves to an internal address is refused whatever
// the DNS answers.

var (
	errBadHost   = errors.New("not a public host name")
	errNotPublic = errors.New("host does not resolve to a public address")
)

// blocked lists the ranges that netip's own predicates do not cover.
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),  // documentation
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),  // reserved, broadcast
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64: embeds an IPv4 address
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"), // documentation
	netip.MustParsePrefix("2002::/16"),     // 6to4: embeds an IPv4 address
}

func isPublic(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// publicOnly is a net.Dialer control hook: it sees the address about to be
// connected to, after name resolution.
func publicOnly(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil || !isPublic(ap.Addr()) {
		return errNotPublic
	}
	return nil
}

// cleanHost extracts a host name from what a visitor typed, which may be a
// bare name or a whole URL. It accepts DNS names only: no IP address, no
// port, no single-label name.
func cleanHost(input string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(input))
	if strings.Contains(h, "://") {
		u, err := url.Parse(h)
		if err != nil {
			return "", errBadHost
		}
		h = u.Hostname()
	} else if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	h = strings.TrimSuffix(h, ".")
	if len(h) == 0 || len(h) > 253 || !strings.Contains(h, ".") {
		return "", errBadHost
	}
	if _, err := netip.ParseAddr(h); err == nil {
		return "", errBadHost
	}
	for _, label := range strings.Split(h, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errBadHost
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return "", errBadHost
			}
		}
	}
	// A name made only of digits and dots is an address in disguise.
	if last := h[strings.LastIndexByte(h, '.')+1:]; strings.Trim(last, "0123456789") == "" {
		return "", errBadHost
	}
	return h, nil
}

// PublicLookup fetches what a public host shows on port 443: the key of its
// certificate and what it negotiates. The certificate chain is verified.
func PublicLookup(ctx context.Context, host string) (stellar.Host, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}
	return stellar.InspectHost(ctx, dialer, net.JoinHostPort(host, "443"), false)
}
