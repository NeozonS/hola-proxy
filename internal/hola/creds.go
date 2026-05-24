package hola

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/log"
)

const defaultListLimit = 3

// CredService bootstraps credentials and rotates them on a timer. It returns:
//   - an AuthProvider closure that always returns the freshest auth header
//   - the initial tunnels response (the caller picks one to dial through)
//   - any startup error (if non-nil, the other two are unset)
//
// If interval > 0, a goroutine is started that re-runs the bootstrap every
// interval and updates the closure. The goroutine has no shutdown channel —
// it is expected to live as long as the process.
func CredService(
	c *Client,
	interval, timeout time.Duration,
	country, proxyType string,
	logger *log.CondLogger,
	backoffInitial, backoffDeadline time.Duration,
) (auth core.AuthProvider, tunnels *ZGetTunnelsResponse, err error) {
	var (
		mux        sync.Mutex
		authHeader string
		userUUID   string
	)
	auth = func() string {
		mux.Lock()
		defer mux.Unlock()
		return authHeader
	}

	txRes, txErr := c.EnsureTransaction(context.Background(), timeout, func(ctx context.Context, hc *http.Client) bool {
		tunnels, userUUID, err = c.Tunnels(ctx, logger, country, proxyType, defaultListLimit, timeout, backoffInitial, backoffDeadline, hc)
		if err != nil {
			logger.Error("Configuration bootstrap error: %v. Retrying with the fallback mechanism...", err)
			return false
		}
		return true
	})
	if txErr != nil {
		logger.Critical("Transaction recovery mechanism failure: %v", txErr)
		err = txErr
		return
	}
	if !txRes {
		logger.Critical("All attempts failed.")
		return
	}

	authHeader = core.BasicAuthHeader(TemplateLogin(userUUID), tunnels.AgentKey)
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			<-ticker.C
			logger.Info("Rotating credentials...")
			var (
				rotErr  error
				tuns    *ZGetTunnelsResponse
				rotUUID string
			)
			rotRes, rotTxErr := c.EnsureTransaction(context.Background(), timeout, func(ctx context.Context, hc *http.Client) bool {
				tuns, rotUUID, rotErr = c.Tunnels(ctx, logger, country, proxyType, defaultListLimit, timeout, backoffInitial, backoffDeadline, hc)
				if rotErr != nil {
					logger.Error("Credential rotation error: %v. Retrying with the fallback mechanism...", rotErr)
					return false
				}
				return true
			})
			if rotTxErr != nil {
				logger.Critical("Transaction recovery mechanism failure: %v", rotTxErr)
				continue
			}
			if !rotRes {
				logger.Critical("All rotation attempts failed.")
				continue
			}
			mux.Lock()
			authHeader = core.BasicAuthHeader(TemplateLogin(rotUUID), tuns.AgentKey)
			mux.Unlock()
			logger.Info("Credentials rotated successfully.")
		}
	}()
	return
}
