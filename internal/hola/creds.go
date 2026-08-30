package hola

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/log"
	"github.com/NeozonS/hola-proxy/internal/random"
)

const (
	defaultListLimit = 3
	// defaultBanCooldown is used when a non-positive ban cooldown is supplied.
	defaultBanCooldown = time.Hour
)

// jitterDuration scales d by a random factor from [0.5, 1.5] so cooldowns
// don't form a perfectly regular request pattern.
func jitterDuration(d time.Duration) time.Duration {
	return time.Duration(float64(d) * (0.5 + rand.New(random.Source).Float64()))
}

// sleepBeforeBanRetry logs and sleeps a jittered cooldown. Hammering the
// Hola API while banned only extends the ban, so pacing is essential.
func sleepBeforeBanRetry(logger *log.CondLogger, cooldown time.Duration) {
	d := jitterDuration(cooldown)
	logger.Warning("cooling down for %v before the next attempt to let the ban lift "+
		"(frequent requests prolong the ban)...", d.Truncate(time.Second))
	time.Sleep(d)
}

// ensureTunnels drives Tunnels until it succeeds, applying ban-aware pacing:
//   - the first ban response triggers a single re-identification (fresh
//     UUID), in case only the UUID itself was banned;
//   - subsequent ban responses trigger long cooldown sleeps instead of
//     tight retry loops;
//   - non-ban failures are returned to the caller.
//
// stopOnPermanent controls whether a persistent permanent ban is returned as
// an error (startup path) or retried forever (background rotation, where the
// old credentials keep being served meanwhile). Returns the UUID actually
// used (it changes on re-identification) and the tunnels response.
func (c *Client) ensureTunnels(
	logger *log.CondLogger,
	country, proxyType string,
	limit uint,
	timeout, backoffInitial, backoffDeadline, banCooldown time.Duration,
	userUUID string,
	stopOnPermanent bool,
) (string, *ZGetTunnelsResponse, error) {
	if banCooldown <= 0 {
		banCooldown = defaultBanCooldown
	}
	reidentified := false
	for {
		var (
			tunnels *ZGetTunnelsResponse
			lastErr error
		)
		txRes, txErr := c.EnsureTransaction(context.Background(), timeout, func(ctx context.Context, hc *http.Client) bool {
			t, err := c.Tunnels(ctx, logger, country, proxyType, limit, timeout, backoffInitial, backoffDeadline, hc, userUUID)
			if err != nil {
				logger.Error("Configuration bootstrap error: %v. Retrying with the fallback mechanism...", err)
				lastErr = err
				return false
			}
			tunnels = t
			return true
		})
		if txErr != nil {
			return userUUID, nil, fmt.Errorf("transaction recovery mechanism failure: %w", txErr)
		}
		if txRes {
			return userUUID, tunnels, nil
		}
		switch {
		case errors.Is(lastErr, PermanentBanError):
			if !reidentified {
				logger.Warning("Hola reported a permanent ban; retrying once with a fresh identity...")
				userUUID = NewUserUUID()
				reidentified = true
				continue
			}
			if stopOnPermanent {
				return userUUID, nil, PermanentBanError
			}
			sleepBeforeBanRetry(logger, banCooldown)
		case errors.Is(lastErr, TemporaryBanError):
			if !reidentified {
				logger.Warning("Hola reported a temporary ban for the current identity; " +
					"retrying once with a fresh identity...")
				userUUID = NewUserUUID()
				reidentified = true
				continue
			}
			sleepBeforeBanRetry(logger, banCooldown)
		default:
			return userUUID, nil, fmt.Errorf("all bootstrap attempts failed, last error: %w", lastErr)
		}
	}
}

// CredService bootstraps credentials and rotates them on a timer. It returns:
//   - an AuthProvider closure that always returns the freshest auth header
//   - the initial tunnels response (the caller picks one to dial through)
//   - any startup error (if non-nil, the other two are unset)
//
// A single random UUID is minted per process and kept stable for the whole
// process lifetime: bootstrap retries and credential rotations re-login with
// the SAME UUID, mimicking the real browser extension. This matters because
// every previously unseen UUID is a new "user registration" from Hola's
// point of view, and registration bursts from one IP are the primary trigger
// for temporary bans. A fresh UUID is generated only when Hola bans the
// current one. Nothing is persisted to disk — the binary is self-contained.
//
// If interval > 0, a goroutine is started that refreshes the credentials
// every interval and updates the closure. The goroutine has no shutdown
// channel — it is expected to live as long as the process.
func CredService(
	c *Client,
	interval, timeout time.Duration,
	country, proxyType string,
	logger *log.CondLogger,
	backoffInitial, backoffDeadline, banCooldown time.Duration,
) (auth core.AuthProvider, tunnels *ZGetTunnelsResponse, err error) {
	var (
		mux        sync.Mutex
		authHeader string
	)
	auth = func() string {
		mux.Lock()
		defer mux.Unlock()
		return authHeader
	}

	userUUID := NewUserUUID()
	userUUID, tunnels, err = c.ensureTunnels(logger, country, proxyType, defaultListLimit,
		timeout, backoffInitial, backoffDeadline, banCooldown, userUUID, true)
	if err != nil {
		logger.Critical("Unable to bootstrap credentials: %v", err)
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
			newUUID, tuns, rerr := c.ensureTunnels(logger, country, proxyType, defaultListLimit,
				timeout, backoffInitial, backoffDeadline, banCooldown, userUUID, false)
			if rerr != nil {
				logger.Error("Credential rotation error: %v. Will retry on the next tick.", rerr)
				continue
			}
			mux.Lock()
			userUUID = newUUID
			authHeader = core.BasicAuthHeader(TemplateLogin(userUUID), tuns.AgentKey)
			mux.Unlock()
			logger.Info("Credentials rotated successfully.")
		}
	}()
	return
}
