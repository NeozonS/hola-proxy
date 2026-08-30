package hola

import (
	"context"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	applog "github.com/NeozonS/hola-proxy/internal/log"
)

func discardLogger() *applog.CondLogger {
	return applog.NewCondLogger(log.New(io.Discard, "", 0), applog.DEBUG)
}

func TestPickFastestSelectsLowestRTT(t *testing.T) {
	eps := []*Endpoint{
		{Host: "1.1.1.1", Port: 22225, TLSName: "slow.hola.org"},
		{Host: "2.2.2.2", Port: 22225, TLSName: "fast.hola.org"},
		{Host: "3.3.3.3", Port: 22225, TLSName: "mid.hola.org"},
	}
	probe := func(_ context.Context, ep *Endpoint) (time.Duration, error) {
		switch ep.TLSName {
		case "slow.hola.org":
			return 400 * time.Millisecond, nil
		case "fast.hola.org":
			return 50 * time.Millisecond, nil
		default:
			return 120 * time.Millisecond, nil
		}
	}
	got, err := PickFastest(context.Background(), discardLogger(), eps, probe)
	if err != nil {
		t.Fatal(err)
	}
	if got.TLSName != "fast.hola.org" {
		t.Fatalf("got %s, want fast.hola.org", got.TLSName)
	}
}

func TestPickFastestSkipsFailures(t *testing.T) {
	eps := []*Endpoint{
		{Host: "1.1.1.1", Port: 22225, TLSName: "dead.hola.org"},
		{Host: "2.2.2.2", Port: 22225, TLSName: "ok.hola.org"},
	}
	probe := func(_ context.Context, ep *Endpoint) (time.Duration, error) {
		if ep.TLSName == "dead.hola.org" {
			return 0, errors.New("dial timeout")
		}
		return 80 * time.Millisecond, nil
	}
	got, err := PickFastest(context.Background(), discardLogger(), eps, probe)
	if err != nil {
		t.Fatal(err)
	}
	if got.TLSName != "ok.hola.org" {
		t.Fatalf("got %s, want ok.hola.org", got.TLSName)
	}
}

func TestPickFastestAllFailed(t *testing.T) {
	eps := []*Endpoint{
		{Host: "1.1.1.1", Port: 22225, TLSName: "a.hola.org"},
	}
	boom := errors.New("no route")
	_, err := PickFastest(context.Background(), discardLogger(), eps, func(context.Context, *Endpoint) (time.Duration, error) {
		return 0, boom
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
}

func TestPickFastestEmpty(t *testing.T) {
	_, err := PickFastest(context.Background(), discardLogger(), nil, func(context.Context, *Endpoint) (time.Duration, error) {
		return 0, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
