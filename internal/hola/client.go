// Package hola talks to Hola's API: enumerates countries, requests tunnels,
// resolves fallback agents and rotates credentials.
//
// All state that used to live in package-level globals (baseDialer, tlsConfig,
// userAgent, cachedFBC) is now held in *Client. Construct one client with
// NewClient(Config{...}) and pass it around.
package hola

import (
	"crypto/x509"
	"net/http"
	"sync"
	"text/template"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/surfclient"
)

const (
	extBrowser      = "chrome"
	product         = "cws"
	ccgiURL         = "https://client.hola.org/client_cgi/"
	vpnCountriesURL = ccgiURL + "vpn_countries.json"
	bgInitURL       = ccgiURL + "background_init"
	zgetTunnelsURL  = ccgiURL + "zgettunnels"
	agentSuffix     = ".hola.org"
	extOrigin       = surfclient.HolaExtOrigin
)

// fallbackConfURLs hold encrypted fallback configurations used when the main
// API endpoint is unreachable. The list is hardcoded by Hola and rotated by
// the project occasionally.
var fallbackConfURLs = []string{
	"https://www.dropbox.com/s/jemizcvpmf2qb9v/cloud_failover.conf?dl=1",
	"https://vdkd6nz8qr.s3.amazonaws.com/cloud_failover.conf",
}

var loginTemplate = template.Must(template.New("LOGIN_TEMPLATE").Parse("user-uuid-{{.uuid}}-is_prem-{{.prem}}"))

// Config carries everything required to construct a Client. Dialer is
// typically set; RootCAs, ExtVer and UserAgent are optional. An empty
// UserAgent keeps the UA from the Windows Chrome impersonate profile so it
// stays aligned with TLS / HTTP2 / sec-ch-ua.
type Config struct {
	UserAgent string
	Dialer    core.ContextDialer
	RootCAs   *x509.CertPool
	ExtVer    string
}

// Client is the per-process Hola API client. It is safe for concurrent use.
type Client struct {
	cfg       Config
	http      *http.Client
	fbcMux    sync.Mutex
	cachedFBC *FallbackConfig
}

// NewClient builds a Client. A single surf Session is kept for the process
// lifetime so background_init and zgettunnels share cookies and TLS tickets.
func NewClient(cfg Config) *Client {
	c := &Client{cfg: cfg}
	c.http = surfclient.New(surfclient.Options{
		Dialer:          cfg.Dialer,
		RootCAs:         cfg.RootCAs,
		UserAgent:       cfg.UserAgent,
		ExtensionOrigin: extOrigin,
		Session:         true,
	})
	return c
}

// UserAgent returns the User-Agent string injected into every Hola API
// request.
func (c *Client) UserAgent() string {
	return c.cfg.UserAgent
}

// ExtVer returns the configured extension version.
func (c *Client) ExtVer() string {
	return c.cfg.ExtVer
}
