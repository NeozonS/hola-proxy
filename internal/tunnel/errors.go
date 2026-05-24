// Package tunnel implements the dialer chain that takes a TCP socket all the
// way through a Hola CONNECT tunnel. It exposes three dialers:
//
//   - ProxyDialer:     CONNECT to a Hola agent over TLS, with hideSNI support
//   - PlaintextDialer: TLS-wrap an existing socket without going through
//                      CONNECT, used for the request-side transport
//   - RetryDialer:     wrap another dialer and re-resolve hostnames blocked
//                      by upstream
package tunnel

import "errors"

// UpstreamBlockedError is returned by ProxyDialer when the Hola agent rejects
// the destination as blocked. RetryDialer recognises this sentinel and tries
// to circumvent the block by resolving the hostname locally and retrying with
// the IP literal.
var UpstreamBlockedError = errors.New("blocked by upstream")
