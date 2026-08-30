package hola

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"time"

	"github.com/NeozonS/hola-proxy/internal/random"
)

// FallbackAgent points at a single fallback Hola agent that the client can
// CONNECT through when the primary client.hola.org endpoint refuses traffic.
type FallbackAgent struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

func (a *FallbackAgent) ToProxy() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(a.Name+agentSuffix, fmt.Sprintf("%d", a.Port)),
	}
}

func (a *FallbackAgent) Hostname() string {
	return a.Name + agentSuffix
}

func (a *FallbackAgent) NetAddr() string {
	return net.JoinHostPort(a.IP, fmt.Sprintf("%d", a.Port))
}

type fallbackConfResponse struct {
	Agents    []FallbackAgent `json:"agents"`
	UpdatedAt int64           `json:"updated_ts"`
	TTL       int64           `json:"ttl_ms"`
}

// FallbackConfig is the decoded fallback agents list with its expiry.
type FallbackConfig struct {
	Agents    []FallbackAgent
	UpdatedAt time.Time
	TTL       time.Duration
}

func (c *FallbackConfig) UnmarshalJSON(data []byte) error {
	r := fallbackConfResponse{}
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	c.Agents = r.Agents
	c.UpdatedAt = time.Unix(r.UpdatedAt/1000, (r.UpdatedAt%1000)*1000000)
	c.TTL = time.Duration(r.TTL * 1000000)
	return nil
}

func (c *FallbackConfig) Expired() bool {
	return time.Now().After(c.UpdatedAt.Add(c.TTL))
}

func (c *FallbackConfig) ShuffleAgents() {
	rand.New(random.Source).Shuffle(len(c.Agents), func(i, j int) {
		c.Agents[i], c.Agents[j] = c.Agents[j], c.Agents[i]
	})
}

func (c *FallbackConfig) Clone() *FallbackConfig {
	return &FallbackConfig{
		Agents:    append([]FallbackAgent(nil), c.Agents...),
		UpdatedAt: c.UpdatedAt,
		TTL:       c.TTL,
	}
}

// fetchFallbackConfig downloads and decodes one of the encrypted fallback
// configs from FALLBACK_CONF_URLS, picked at random. The encoding is a
// "rotate-by-3-then-base64" obfuscation.
func (c *Client) fetchFallbackConfig(ctx context.Context) (*FallbackConfig, error) {
	client := c.httpClientWithProxy(nil)
	url := fallbackConfURLs[rand.New(random.Source).Intn(len(fallbackConfURLs))]
	confRaw, err := c.doReq(ctx, client, "", url, nil, nil)
	if err != nil {
		return nil, err
	}

	l := len(confRaw)
	if l < 4 {
		return nil, errors.New("bad response length from fallback conf URL")
	}

	buf := &bytes.Buffer{}
	buf.Grow(l)
	buf.Write(confRaw[l-3:])
	buf.Write(confRaw[:l-3])

	b64dec := base64.NewDecoder(base64.RawStdEncoding, buf)
	jdec := json.NewDecoder(b64dec)
	fbc := &FallbackConfig{}
	if err := jdec.Decode(fbc); err != nil {
		return nil, err
	}
	if fbc.Expired() {
		return nil, errors.New("fetched expired fallback config")
	}
	fbc.ShuffleAgents()
	return fbc, nil
}

// GetFallbackProxies returns a cached fallback config, refreshing it from the
// network if the cached one is missing or expired. The returned value is a
// clone, so callers can mutate the agent order safely.
func (c *Client) GetFallbackProxies(ctx context.Context) (*FallbackConfig, error) {
	c.fbcMux.Lock()
	defer c.fbcMux.Unlock()

	if c.cachedFBC == nil || c.cachedFBC.Expired() {
		fbc, err := c.fetchFallbackConfig(ctx)
		if err != nil {
			return nil, err
		}
		c.cachedFBC = fbc
	}
	return c.cachedFBC.Clone(), nil
}
