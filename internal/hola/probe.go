package hola

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NeozonS/hola-proxy/internal/log"
)

// ProbeFunc measures how long it takes to make one endpoint usable.
// The duration should cover a real tunnel setup, not just a TCP SYN.
type ProbeFunc func(ctx context.Context, ep *Endpoint) (time.Duration, error)

type probeResult struct {
	ep  *Endpoint
	rtt time.Duration
	err error
}

// PickFastest probes every endpoint in parallel and returns the one with
// the lowest successful setup time. Endpoints that fail the probe are
// skipped. If every probe fails, the first error is returned.
func PickFastest(ctx context.Context, logger *log.CondLogger, endpoints []*Endpoint, probe ProbeFunc) (*Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no endpoints to probe")
	}
	if probe == nil {
		return nil, fmt.Errorf("probe function is nil")
	}

	results := make([]probeResult, len(endpoints))
	var wg sync.WaitGroup
	wg.Add(len(endpoints))
	for i, ep := range endpoints {
		i, ep := i, ep
		go func() {
			defer wg.Done()
			rtt, err := probe(ctx, ep)
			results[i] = probeResult{ep: ep, rtt: rtt, err: err}
		}()
	}
	wg.Wait()

	var (
		best     *Endpoint
		bestRTT  time.Duration
		firstErr error
		okCount  int
	)
	for _, r := range results {
		if r.err != nil {
			if logger != nil {
				logger.Warning("endpoint %s probe failed: %v", r.ep.URL().String(), r.err)
			}
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		okCount++
		if logger != nil {
			logger.Info("endpoint %s probed in %s", r.ep.URL().String(), r.rtt.Truncate(time.Millisecond))
		}
		if best == nil || r.rtt < bestRTT {
			best = r.ep
			bestRTT = r.rtt
		}
	}
	if best == nil {
		return nil, fmt.Errorf("all %d endpoints failed probe: %w", len(endpoints), firstErr)
	}
	if logger != nil {
		logger.Info("selected fastest endpoint %s (%s, %d/%d reachable)",
			best.URL().String(), bestRTT.Truncate(time.Millisecond), okCount, len(endpoints))
	}
	return best, nil
}
