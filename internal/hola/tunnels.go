package hola

import (
	"context"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"

	"github.com/NeozonS/hola-proxy/internal/log"
)

// Countries returns the list of country codes Hola advertises. The returned
// list is deduplicated and sorted; "uk" gets a "gb" alias appended for
// convenience.
func (c *Client) Countries(ctx context.Context, hc *http.Client) (CountryList, error) {
	return c.vpnCountries(ctx, hc)
}

// Tunnels performs the full bootstrap dance for a country/proxy_type combo:
// 1. issue a fresh user UUID
// 2. POST background_init to obtain a session_key
// 3. POST zgettunnels with exponential backoff until either it succeeds or
//    the deadline is hit
//
// Returns the tunnels response, the user UUID used (caller needs it to build
// auth headers), and the last error if the whole process failed.
func (c *Client) Tunnels(ctx context.Context, logger *log.CondLogger, country, proxyType string, limit uint, timeout, backoffInitial, backoffDeadline time.Duration, hc *http.Client) (*ZGetTunnelsResponse, string, error) {
	u := uuid.New()
	userUUID := hex.EncodeToString(u[:])

	ctx1, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	initRes, err := c.backgroundInit(ctx1, hc, userUUID)
	if err != nil {
		return nil, "", err
	}

	var bo backoff.BackOff = &backoff.ExponentialBackOff{
		InitialInterval:     backoffInitial,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         10 * time.Minute,
		MaxElapsedTime:      backoffDeadline,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	bo = backoff.WithContext(bo, ctx)

	var res *ZGetTunnelsResponse
	var lastErr error
	err = backoff.RetryNotify(func() error {
		ctxIter, cancelIter := context.WithTimeout(ctx, timeout)
		defer cancelIter()
		var rerr error
		res, rerr = c.zgetTunnels(ctxIter, hc, userUUID, initRes.Key, country, proxyType, limit)
		lastErr = rerr
		return rerr
	}, bo, func(err error, dur time.Duration) {
		logger.Info("zgettunnels error: %v; will retry after %v", err, dur.Truncate(time.Millisecond))
	})
	if err != nil {
		logger.Error("All attempts failed: %v", err)
		return nil, "", err
	}
	_ = lastErr
	return res, userUUID, nil
}
