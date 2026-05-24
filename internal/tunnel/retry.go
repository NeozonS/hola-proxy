package tunnel

import (
	"context"
	"net"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/dns"
	"github.com/NeozonS/hola-proxy/internal/log"
)

// RetryDialer wraps another dialer and falls back to local DNS resolution if
// the inner dialer reports UpstreamBlockedError — Hola sometimes refuses to
// resolve a hostname but happily proxies traffic to its IP.
type RetryDialer struct {
	dialer   core.ContextDialer
	resolver *dns.Resolver
	logger   *log.CondLogger
}

func NewRetryDialer(dialer core.ContextDialer, resolver *dns.Resolver, logger *log.CondLogger) *RetryDialer {
	return &RetryDialer{dialer: dialer, resolver: resolver, logger: logger}
}

func (d *RetryDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dialer.DialContext(ctx, network, address)
	if err == UpstreamBlockedError {
		d.logger.Info("Destination %s blocked by upstream. Rescuing it with resolve&tunnel workaround.", address)
		host, port, err1 := net.SplitHostPort(address)
		if err1 != nil {
			return conn, err
		}
		ips := d.resolver.Resolve(host)
		if len(ips) == 0 {
			return conn, err
		}
		return d.dialer.DialContext(ctx, network, net.JoinHostPort(ips[0], port))
	}
	return conn, err
}

func (d *RetryDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}
