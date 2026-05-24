// Package core defines small interfaces and helpers shared across the project.
// It is a leaf package — it must not import any other internal package.
package core

import (
	"context"
	"net"
)

// Dialer is the minimal blocking dialer interface.
type Dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// ContextDialer extends Dialer with context-aware dialing. All proxy chains
// in this project speak ContextDialer.
type ContextDialer interface {
	Dialer
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}
