// Package dns provides a thin wrapper around AdGuard's upstream resolver
// supporting plain DNS, DoH, DoT and DoQ via the dnsproxy library.
package dns

import (
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	"github.com/miekg/dns"
)

const dot = 0x2e

// Resolver is the project-level DNS resolver. It resolves A then AAAA, and
// returns string IPs (not net.IP) because callers feed them straight into
// net.JoinHostPort.
type Resolver struct {
	upstream upstream.Upstream
}

// NewResolver builds a resolver for the given address. address follows the
// format used by github.com/ameshkov/dnslookup (e.g. "https://cloudflare-dns.com/dns-query").
func NewResolver(address string, timeout time.Duration) (*Resolver, error) {
	opts := &upstream.Options{Timeout: timeout}
	u, err := upstream.AddressToUpstream(address, opts)
	if err != nil {
		return nil, err
	}
	return &Resolver{upstream: u}, nil
}

// ResolveA returns IPv4 addresses for the given domain.
func (r *Resolver) ResolveA(domain string) []string {
	return r.resolve(domain, dns.TypeA, func(rr dns.RR) (string, bool) {
		a, ok := rr.(*dns.A)
		if !ok {
			return "", false
		}
		return a.A.String(), true
	})
}

// ResolveAAAA returns IPv6 addresses for the given domain.
func (r *Resolver) ResolveAAAA(domain string) []string {
	return r.resolve(domain, dns.TypeAAAA, func(rr dns.RR) (string, bool) {
		a, ok := rr.(*dns.AAAA)
		if !ok {
			return "", false
		}
		return a.AAAA.String(), true
	})
}

// Resolve returns IPv4 addresses, falling back to IPv6 if no IPv4 is available.
func (r *Resolver) Resolve(domain string) []string {
	res := r.ResolveA(domain)
	if len(res) == 0 {
		res = r.ResolveAAAA(domain)
	}
	return res
}

func (r *Resolver) resolve(domain string, qtype uint16, extract func(dns.RR) (string, bool)) []string {
	res := make([]string, 0)
	if len(domain) == 0 {
		return res
	}
	if domain[len(domain)-1] != dot {
		domain = domain + "."
	}
	req := dns.Msg{}
	req.Id = dns.Id()
	req.RecursionDesired = true
	req.Question = []dns.Question{{Name: domain, Qtype: qtype, Qclass: dns.ClassINET}}
	reply, err := r.upstream.Exchange(&req)
	if err != nil {
		return res
	}
	for _, rr := range reply.Answer {
		if v, ok := extract(rr); ok {
			res = append(res, v)
		}
	}
	return res
}
