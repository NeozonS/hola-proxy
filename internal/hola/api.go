package hola

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/campoy/unique"

	"github.com/NeozonS/hola-proxy/internal/random"
	"github.com/NeozonS/hola-proxy/internal/surfclient"
	"github.com/NeozonS/hola-proxy/internal/tunnel"
)

// httpClientWithProxy returns an *http.Client whose transport is a
// surf-backed Chrome-impersonating client. If agent is non-nil, the outgoing
// connection is first tunneled through that fallback agent (HTTPS CONNECT
// with hideSNI), and only then the surf client performs its own outer TLS
// handshake to client.hola.org.
func (c *Client) httpClientWithProxy(agent *FallbackAgent) *http.Client {
	dialer := c.cfg.Dialer
	rootCAs := c.cfg.RootCAs
	if agent != nil {
		dialer = tunnel.NewProxyDialer(agent.NetAddr(), agent.Hostname(), rootCAs, nil, true, dialer)
	}
	return surfclient.New(dialer, rootCAs)
}

// doReq is a tiny request helper shared by the API methods. Adds the
// project-wide User-Agent and treats any non-2xx response as an error.
func (c *Client) doReq(ctx context.Context, client *http.Client, method, rawURL string, query, data url.Values) ([]byte, error) {
	if method == "" {
		method = "GET"
	}
	var (
		req *http.Request
		err error
	)
	if data == nil {
		req, err = http.NewRequestWithContext(ctx, method, rawURL, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader([]byte(data.Encode())))
	}
	if err != nil {
		return nil, err
	}
	if data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
	default:
		return nil, fmt.Errorf("Bad HTTP response: %s", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return body, nil
}

// vpnCountries lists countries (per-client implementation, so it sees the
// configured user agent and dialer).
func (c *Client) vpnCountries(ctx context.Context, hc *http.Client) (CountryList, error) {
	params := make(url.Values)
	params.Add("browser", extBrowser)
	data, err := c.doReq(ctx, hc, "", vpnCountriesURL, params, nil)
	if err != nil {
		return nil, err
	}
	var res CountryList
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, err
	}
	for _, a := range res {
		if a == "uk" {
			res = append(res, "gb")
		}
	}
	less := func(i, j int) bool { return res[i] < res[j] }
	unique.Slice(&res, less)
	return res, nil
}

func (c *Client) backgroundInit(ctx context.Context, hc *http.Client, userUUID string) (BgInitResponse, error) {
	postData := make(url.Values)
	postData.Add("login", "1")
	postData.Add("ver", c.cfg.ExtVer)
	qs := make(url.Values)
	qs.Add("uuid", userUUID)
	resp, err := c.doReq(ctx, hc, "POST", bgInitURL, qs, postData)
	if err != nil {
		return BgInitResponse{}, err
	}
	var res BgInitResponse
	if err := json.Unmarshal(resp, &res); err != nil {
		return res, err
	}
	if res.Blocked {
		if res.Permanent {
			return res, PermanentBanError
		}
		return res, TemporaryBanError
	}
	return res, nil
}

func (c *Client) zgetTunnels(ctx context.Context, hc *http.Client, userUUID string, sessionKey int64, country, proxyType string, limit uint) (*ZGetTunnelsResponse, error) {
	params := make(url.Values)
	switch proxyType {
	case "lum":
		params.Add("country", country+".pool_lum_"+country+"_shared")
	case "virt":
		params.Add("country", country+".pool_virt_pool_"+country)
	case "peer":
		params.Add("country", country)
	case "pool":
		params.Add("country", country+".pool")
	default: // direct or skip
		params.Add("country", country)
	}
	params.Add("limit", strconv.FormatInt(int64(limit), 10))
	params.Add("ping_id", strconv.FormatFloat(rand.New(random.Source).Float64(), 'f', -1, 64))
	params.Add("ext_ver", c.cfg.ExtVer)
	params.Add("browser", extBrowser)
	params.Add("product", product)
	params.Add("uuid", userUUID)
	params.Add("session_key", strconv.FormatInt(sessionKey, 10))
	params.Add("is_premium", "0")
	data, err := c.doReq(ctx, hc, "POST", zgetTunnelsURL, params, nil)
	if err != nil {
		return nil, err
	}
	var tunnels ZGetTunnelsResponse
	if err := json.Unmarshal(data, &tunnels); err != nil {
		return nil, fmt.Errorf("unable to unmashal zgettunnels response: %w", err)
	}
	if len(tunnels.IPList) == 0 {
		return nil, EmptyResponseError
	}
	return &tunnels, nil
}

// EnsureTransaction runs txn with a primary surf client, and on failure
// retries with each fallback agent in turn. It is the single retry/fallback
// orchestration point for this package.
func (c *Client) EnsureTransaction(ctx context.Context, getFBTimeout time.Duration, txn func(context.Context, *http.Client) bool) (bool, error) {
	client := c.httpClientWithProxy(nil)
	defer client.CloseIdleConnections()

	if txn(ctx, client) {
		return true, nil
	}

	fbCtx, cancel := context.WithTimeout(ctx, getFBTimeout)
	defer cancel()
	fbc, err := c.GetFallbackProxies(fbCtx)
	if err != nil {
		return false, err
	}

	for _, agent := range fbc.Agents {
		client = c.httpClientWithProxy(&agent)
		defer client.CloseIdleConnections()
		if txn(ctx, client) {
			return true, nil
		}
	}
	return false, nil
}
