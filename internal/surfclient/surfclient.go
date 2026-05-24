// Package surfclient produces *net/http.Client instances backed by
// github.com/enetx/surf with Chrome browser impersonation enabled.
//
// Using surf gives a modern TLS JA3 fingerprint, HTTP/2 settings and
// browser-accurate header ordering — but the returned client is a standard
// net/http.Client, so consumers in this project keep using stdlib types.
package surfclient

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"

	enethttp "github.com/enetx/http"
	"github.com/enetx/surf"

	"github.com/NeozonS/hola-proxy/internal/core"
)

// New builds an *http.Client that uses surf under the hood.
//
// If dialer is non-nil, it is injected into the underlying transport's
// DialContext via a client middleware that runs before surf's JA3 round
// tripper wraps the transport, so the dialer is preserved through wrapping.
//
// If rootCAs is non-nil, it is set on the TLS config used by surf, honoring
// the project's -cafile flag.
//
// Certificate verification is enabled (SecureTLS). Surf's default is to skip
// verification, which would be a regression compared to the original net/http
// based code.
func New(dialer core.ContextDialer, rootCAs *x509.CertPool) *http.Client {
	builder := surf.NewClient().
		Builder().
		Impersonate().Chrome().
		SecureTLS()

	// Priority 0; JA wrapper uses math.MaxInt, so this runs first.
	builder = builder.With(func(c *surf.Client) error {
		if dialer != nil {
			if t, ok := c.GetTransport().(*enethttp.Transport); ok {
				t.DialContext = dialer.DialContext
			}
		}
		if rootCAs != nil {
			cfg := c.GetTLSConfig()
			if cfg == nil {
				cfg = &tls.Config{}
			}
			cfg.RootCAs = rootCAs
		}
		return nil
	})

	sc := builder.Build().Unwrap()
	return sc.Std()
}
