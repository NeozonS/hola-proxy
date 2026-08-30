package hola

import (
	"cmp"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
)

// Endpoint describes a single Hola tunnel endpoint: TCP target + optional
// TLS server name (when non-empty, the tunnel uses TLS).
type Endpoint struct {
	Host    string
	Port    uint16
	TLSName string
}

func (e *Endpoint) URL() *url.URL {
	if e.TLSName == "" {
		return &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port)),
		}
	}
	return &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(e.TLSName, fmt.Sprintf("%d", e.Port)),
	}
}

func (e *Endpoint) NetAddr() string {
	return net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port))
}

func resolvePort(tunnels *ZGetTunnelsResponse, typ string, trial bool, forcePortField string) (uint16, error) {
	if forcePortField != "" {
		port2, err := strconv.ParseUint(forcePortField, 0, 16)
		if err == nil {
			return uint16(port2), nil
		}
		typ = forcePortField
	}
	switch typ {
	case "direct", "lum", "pool", "virt":
		if !trial {
			return tunnels.Port.Trial, nil
		}
		return tunnels.Port.Direct, nil
	case "peer":
		if !trial {
			return tunnels.Port.TrialPeer, nil
		}
		return tunnels.Port.Peer, nil
	default:
		return 0, errors.New("Unsupported port type")
	}
}

// Endpoints returns every agent in the tunnels response, with the port that
// matches Hola's mode (trial/direct/peer). forcePortField overrides the port
// when it is a number, or the mode name when it is a string. The slice is
// sorted by TLSName so callers can probe in a stable order.
func Endpoints(tunnels *ZGetTunnelsResponse, typ string, trial bool, forcePortField string) ([]*Endpoint, error) {
	if tunnels == nil || len(tunnels.IPList) == 0 {
		return nil, errors.New("No tunnels found in API response")
	}
	port, err := resolvePort(tunnels, typ, trial, forcePortField)
	if err != nil {
		return nil, err
	}
	out := make([]*Endpoint, 0, len(tunnels.IPList))
	for hostname, ip := range tunnels.IPList {
		if hostname == "" || ip == "" {
			continue
		}
		out = append(out, &Endpoint{
			Host:    ip,
			Port:    port,
			TLSName: hostname,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("No tunnels found in API response")
	}
	slices.SortFunc(out, func(a, b *Endpoint) int {
		return cmp.Compare(a.TLSName, b.TLSName)
	})
	return out, nil
}

// GetEndpoint picks one endpoint from a tunnels response. Prefer Endpoints
// plus PickFastest at startup; this keeps the old "first of the list" behavior.
func GetEndpoint(tunnels *ZGetTunnelsResponse, typ string, trial bool, forcePortField string) (*Endpoint, error) {
	endpoints, err := Endpoints(tunnels, typ, trial, forcePortField)
	if err != nil {
		return nil, err
	}
	return endpoints[0], nil
}
