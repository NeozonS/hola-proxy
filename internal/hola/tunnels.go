package hola

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"

	"github.com/NeozonS/hola-proxy/internal/log"
)

// NewUserUUID generates a fresh random Hola user UUID (hex, no dashes).
// Callers should prefer reusing an existing UUID (see CachedCredentials):
// every new UUID is a new "user registration" from Hola's point of view.
func NewUserUUID() string {
	u := uuid.New()
	return hex.EncodeToString(u[:])
}

// Countries returns the list of country codes Hola advertises. The returned
// list is deduplicated and sorted; "uk" gets a "gb" alias appended for
// convenience.
func (c *Client) Countries(ctx context.Context, hc *http.Client) (CountryList, error) {
	return c.vpnCountries(ctx, hc)
}

// Tunnels performs the full bootstrap dance for a country/proxy_type combo:
//  1. POST background_init with the given user UUID to obtain a session_key
//  2. POST zgettunnels with exponential backoff until either it succeeds or
//     the deadline is hit
//
// The UUID is supplied by the caller so identities can be kept stable across
// restarts and rotations; use NewUserUUID to mint a fresh one. Ban responses
// are not retried here (the caller applies ban-aware pacing); everything else
// is retried with exponential backoff until backoffDeadline.
//
// Returns the tunnels response and the last error if the process failed.
func (c *Client) Tunnels(ctx context.Context, logger *log.CondLogger, country, proxyType string, limit uint, timeout, backoffInitial, backoffDeadline time.Duration, hc *http.Client, userUUID string) (*ZGetTunnelsResponse, error) {
	ctx1, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	initRes, err := c.backgroundInit(ctx1, hc, userUUID)
	if err != nil {
		return nil, err
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
		if errors.Is(rerr, TemporaryBanError) || errors.Is(rerr, PermanentBanError) {
			// Hammering the API while banned only extends the ban; bail out
			// immediately and let the caller pace retries with a cooldown.
			return backoff.Permanent(rerr)
		}
		return rerr
	}, bo, func(err error, dur time.Duration) {
		logger.Info("zgettunnels error: %v; will retry after %v", err, dur.Truncate(time.Millisecond))
	})
	if err != nil {
		logger.Error("All attempts failed: %v", err)
		return nil, err
	}
	_ = lastErr
	return res, nil
}
