package hola

import (
	"errors"
	"fmt"
	"net"
	"net/url"
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

// GetEndpoint picks one endpoint from a tunnels response. The selection rules
// match Hola's port semantics: the same agent exposes different ports for
// "direct"/"peer"/"hola"/"trial"/"trial_peer" usage. forcePortField overrides
// everything if non-empty.
func GetEndpoint(tunnels *ZGetTunnelsResponse, typ string, trial bool, forcePortField string) (*Endpoint, error) {
	var hostname, ip string
	for k, v := range tunnels.IPList {
		hostname = k
		ip = v
		break
	}
	if hostname == "" || ip == "" {
		return nil, errors.New("No tunnels found in API response")
	}

	var port uint16
	if forcePortField != "" {
		port2, err := strconv.ParseUint(forcePortField, 0, 16)
		if err == nil {
			port = uint16(port2)
			typ = "skip"
		} else {
			typ = forcePortField
		}
	}
	if typ != "skip" {
		switch typ {
		case "direct", "lum", "pool", "virt":
			if !trial {
				port = tunnels.Port.Trial
			} else {
				port = tunnels.Port.Direct
			}
		case "peer":
			if !trial {
				port = tunnels.Port.TrialPeer
			} else {
				port = tunnels.Port.Peer
			}
		default:
			return nil, errors.New("Unsupported port type")
		}
	}
	return &Endpoint{
		Host:    ip,
		Port:    port,
		TLSName: hostname,
	}, nil
}
