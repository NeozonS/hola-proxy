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
	"strings"

	enethttp "github.com/enetx/http"
	"github.com/enetx/surf"
	"github.com/enetx/surf/header"
	"github.com/enetx/surf/profiles"
	"github.com/enetx/surf/profiles/chrome"

	"github.com/NeozonS/hola-proxy/internal/core"
)

// HolaExtOrigin is the Chrome Web Store origin of the official Hola
// extension. Background fetches from that extension send this as Origin.
const HolaExtOrigin = "chrome-extension://gkojfkhlekighikafcpjkiklfbnlmeio"

// ChromeWindowsUA is the User-Agent of the Windows Chrome profile that
// Impersonate().Windows().Chrome() applies (TLS / HTTP2 / sec-ch-ua stay in
// lockstep with this string). Do not replace it with a "latest Chrome" scrape:
// a newer UA on an older ClientHello is a bot tell.
func ChromeWindowsUA() string {
	return chrome.UserAgent.Get(profiles.Windows).UnwrapOrDefault().Std()
}

// ChromeProdVersion is the dotted Chrome version taken from ChromeWindowsUA,
// suitable as the Chrome Web Store `prodversion` query parameter.
func ChromeProdVersion() string {
	ua := ChromeWindowsUA()
	_, rest, ok := strings.Cut(ua, "Chrome/")
	if !ok {
		return "150.0.0.0"
	}
	ver, _, _ := strings.Cut(rest, " ")
	if ver == "" {
		return "150.0.0.0"
	}
	return ver
}

// Options configures a surf-backed HTTP client.
type Options struct {
	Dialer    core.ContextDialer
	RootCAs   *x509.CertPool
	UserAgent string
	// ExtensionOrigin, if set, rewrites fetch metadata after impersonate so
	// the request looks like a Chrome extension service worker (Hola's real
	// client) rather than a page navigation.
	ExtensionOrigin string
	Session         bool
}

// New builds an *http.Client that uses surf under the hood.
//
// TLS JA3, HTTP/2 settings and header order come from Chrome-on-Windows.
// Certificate verification is enabled (SecureTLS).
func New(opts Options) *http.Client {
	builder := surf.NewClient().Builder()
	if opts.Session {
		builder = builder.Session()
	}
	builder = builder.Impersonate().Windows().Chrome().SecureTLS()
	if opts.UserAgent != "" {
		builder = builder.UserAgent(opts.UserAgent)
	}

	// Priority 0; JA wrapper uses math.MaxInt, so this runs first.
	builder = builder.With(func(c *surf.Client) error {
		if opts.Dialer != nil {
			if t, ok := c.GetTransport().(*enethttp.Transport); ok {
				t.DialContext = opts.Dialer.DialContext
			}
		}
		if opts.RootCAs != nil {
			cfg := c.GetTLSConfig()
			if cfg == nil {
				cfg = &tls.Config{}
			}
			cfg.RootCAs = opts.RootCAs
		}
		return nil
	})

	if opts.ExtensionOrigin != "" {
		origin := opts.ExtensionOrigin
		// Impersonate SetHeaders is priority 0; run after it so we replace
		// page-like Sec-Fetch-* / empty Origin with extension-SW values.
		builder = builder.With(func(req *surf.Request) error {
			h := req.GetRequest().Header
			h.Set("Origin", origin)
			h.Set(header.SEC_FETCH_SITE, "none")
			h.Set(header.SEC_FETCH_MODE, "cors")
			h.Set(header.SEC_FETCH_DEST, "empty")
			h.Set("Accept", "*/*")
			h.Del("Referer")
			h.Del("Upgrade-Insecure-Requests")
			h.Del(header.SEC_FETCH_USER)
			return nil
		}, 100)
	}

	return builder.Build().Unwrap().Std()
}
