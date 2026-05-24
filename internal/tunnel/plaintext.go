package tunnel

import (
	"context"
	"crypto/x509"
	"errors"
	"net"

	utls "github.com/refraction-networking/utls"

	"github.com/NeozonS/hola-proxy/internal/core"
)

// PlaintextDialer wraps an existing TCP connection in a TLS session to a fixed
// address. It does NOT perform a CONNECT handshake — it's used for the
// request-side transport in the proxy handler, where the CONNECT-equivalent
// happens at the HTTP layer (Proxy-Authorization header).
type PlaintextDialer struct {
	fixedAddress  string
	tlsServerName string
	next          core.ContextDialer
	caPool        *x509.CertPool
	hideSNI       bool
}

func NewPlaintextDialer(address, tlsServerName string, caPool *x509.CertPool, hideSNI bool, next core.ContextDialer) *PlaintextDialer {
	return &PlaintextDialer{
		fixedAddress:  address,
		tlsServerName: tlsServerName,
		next:          next,
		caPool:        caPool,
		hideSNI:       hideSNI,
	}
}

func (d *PlaintextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, errors.New("bad network specified for DialContext: only tcp is supported")
	}

	conn, err := d.next.DialContext(ctx, "tcp", d.fixedAddress)
	if err != nil {
		return nil, err
	}

	if d.tlsServerName == "" {
		return conn, nil
	}

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
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (d *PlaintextDialer) Dial(network, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}
