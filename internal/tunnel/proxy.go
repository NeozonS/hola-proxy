package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	utls "github.com/refraction-networking/utls"

	"github.com/NeozonS/hola-proxy/internal/core"
)

const (
	proxyConnectMethod       = "CONNECT"
	proxyHostHeader          = "Host"
	proxyAuthorizationHeader = "Proxy-Authorization"
)

// ProxyDialer establishes a CONNECT tunnel through a Hola HTTPS proxy.
// Optionally hides SNI in the outer TLS handshake.
type ProxyDialer struct {
	address       string
	tlsServerName string
	auth          core.AuthProvider
	next          core.ContextDialer
	caPool        *x509.CertPool
	hideSNI       bool
}

func NewProxyDialer(address, tlsServerName string, caPool *x509.CertPool, auth core.AuthProvider, hideSNI bool, nextDialer core.ContextDialer) *ProxyDialer {
	return &ProxyDialer{
		address:       address,
		tlsServerName: tlsServerName,
		auth:          auth,
		next:          nextDialer,
		caPool:        caPool,
		hideSNI:       hideSNI,
	}
}

// ProxyDialerFromURL turns http(s)://[user:pass@]host[:port] into a ProxyDialer.
// Used for the -proxy chain flag.
func ProxyDialerFromURL(u *url.URL, caPool *x509.CertPool, next core.ContextDialer) (*ProxyDialer, error) {
	host := u.Hostname()
	port := u.Port()
	tlsServerName := ""
	var auth core.AuthProvider

	switch strings.ToLower(u.Scheme) {
	case "http":
		if port == "" {
			port = "80"
		}
	case "https":
		if port == "" {
			port = "443"
		}
		tlsServerName = host
	default:
		return nil, errors.New("unsupported proxy type")
	}

	address := net.JoinHostPort(host, port)

	if u.User != nil {
		username := u.User.Username()
		password, _ := u.User.Password()
		authHeader := core.BasicAuthHeader(username, password)
		auth = func() string {
			return authHeader
		}
	}
	return NewProxyDialer(address, tlsServerName, caPool, auth, false, next), nil
}

func (d *ProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.New("bad network specified for DialContext: only tcp is supported")
	}

	conn, err := d.next.DialContext(ctx, "tcp", d.address)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			conn.Close()
		}
	}()

	if d.tlsServerName != "" {
		// Custom cert verification logic:
		// DO NOT send SNI extension of TLS ClientHello
		// DO peer certificate verification against specified servername
		sni := d.tlsServerName
		if d.hideSNI {
			sni = ""
		}
		tlsConn := utls.UClient(conn, &utls.Config{
			ServerName:         sni,
			InsecureSkipVerify: true,
			VerifyConnection: func(cs utls.ConnectionState) error {
				opts := x509.VerifyOptions{
					DNSName:       d.tlsServerName,
					Intermediates: x509.NewCertPool(),
					Roots:         d.caPool,
				}
				for _, cert := range cs.PeerCertificates[1:] {
					opts.Intermediates.AddCert(cert)
				}
				_, err := cs.PeerCertificates[0].Verify(opts)
				return err
			},
		}, utls.HelloChrome_Auto)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	req := &http.Request{
		Method:     proxyConnectMethod,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		RequestURI: address,
		Host:       address,
		Header: http.Header{
			proxyHostHeader: []string{address},
		},
	}

	if d.auth != nil {
		req.Header.Set(proxyAuthorizationHeader, d.auth())
	}

	rawreq, err := httputil.DumpRequest(req, false)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(rawreq); err != nil {
		return nil, err
	}

	proxyResp, err := readResponse(conn, req)
	if err != nil {
		return nil, err
	}

	if proxyResp.StatusCode != http.StatusOK {
		if proxyResp.StatusCode == http.StatusForbidden &&
			proxyResp.Header.Get("X-Hola-Error") == "Forbidden Host" {
			return nil, UpstreamBlockedError
		}
		return nil, fmt.Errorf("bad response from upstream proxy server: %s", proxyResp.Status)
	}
	ok = true
	return conn, nil
}

func (d *ProxyDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// readResponse drains bytes one at a time until \r\n\r\n, then parses the
// response. We can't use http.ReadResponse directly because the connection
// may be reused for the tunnel afterwards.
func readResponse(r io.Reader, req *http.Request) (*http.Response, error) {
	endOfResponse := []byte("\r\n\r\n")
	buf := &bytes.Buffer{}
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n < 1 && err == nil {
			continue
		}
		buf.Write(b)
		sl := buf.Bytes()
		if len(sl) < len(endOfResponse) {
			continue
		}
		if bytes.Equal(sl[len(sl)-4:], endOfResponse) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	return http.ReadResponse(bufio.NewReader(buf), req)
}
