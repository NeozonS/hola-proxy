// Package hola talks to Hola's API: enumerates countries, requests tunnels,
// resolves fallback agents and rotates credentials.
//
// All state that used to live in package-level globals (baseDialer, tlsConfig,
// userAgent, cachedFBC) is now held in *Client. Construct one client with
// NewClient(Config{...}) and pass it around.
package hola

import (
	"crypto/x509"
	"sync"
	"text/template"

	"github.com/NeozonS/hola-proxy/internal/core"
)

const (
	extBrowser       = "chrome"
	product          = "cws"
	ccgiURL          = "https://client.hola.org/client_cgi/"
	vpnCountriesURL  = ccgiURL + "vpn_countries.json"
	bgInitURL        = ccgiURL + "background_init"
	zgetTunnelsURL   = ccgiURL + "zgettunnels"
	agentSuffix      = ".hola.org"
)

// fallbackConfURLs hold encrypted fallback configurations used when the main
// API endpoint is unreachable. The list is hardcoded by Hola and rotated by
// the project occasionally.
var fallbackConfURLs = []string{
	"https://www.dropbox.com/s/jemizcvpmf2qb9v/cloud_failover.conf?dl=1",
	"https://vdkd6nz8qr.s3.amazonaws.com/cloud_failover.conf",
}

var loginTemplate = template.Must(template.New("LOGIN_TEMPLATE").Parse("user-uuid-{{.uuid}}-is_prem-{{.prem}}"))

// Config carries everything required to construct a Client. UserAgent and
// Dialer are mandatory; RootCAs and ExtVer are optional but typically set.
type Config struct {
	UserAgent string
	Dialer    core.ContextDialer
	RootCAs   *x509.CertPool
	ExtVer    string
}

// Client is the per-process Hola API client. It is safe for concurrent use.
type Client struct {
	cfg       Config
	fbcMux    sync.Mutex
	cachedFBC *FallbackConfig
}

// NewClient builds a Client. The caller is expected to populate cfg.UserAgent
// and cfg.Dialer; if Dialer is nil, surf's default dialer is used.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
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
