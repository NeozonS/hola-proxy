package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/hola"
	applog "github.com/NeozonS/hola-proxy/internal/log"
)

// fetchCountries retrieves the list of countries Hola advertises, retrying
// with the fallback mechanism on failure.
func fetchCountries(client *hola.Client, try func(string, func() error) error, timeout time.Duration) (hola.CountryList, error) {
	var (
		countries hola.CountryList
		err       error
		txRes     bool
		txErr     error
	)
	err = try("list VPN countries", func() error {
		txRes, txErr = client.EnsureTransaction(context.Background(), timeout, func(ctx context.Context, hc *http.Client) bool {
			ctx1, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			countries, err = client.Countries(ctx1, hc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Transaction error: %v. Retrying with the fallback mechanism...\n", err)
				return false
			}
			return true
		})
		if txErr != nil {
			return fmt.Errorf("transaction recovery mechanism failure: %w", txErr)
		}
		if !txRes {
			return errors.New("all fallback proxies failed")
		}
		return nil
	})
	return countries, err
}

// printCountries prints `<code> - <name>` lines for every country Hola
// advertises, then returns a process exit code.
func printCountries(client *hola.Client, try func(string, func() error) error, timeout time.Duration) int {
	countries, err := fetchCountries(client, try, timeout)
	if err != nil {
		return 3
	}
	for _, code := range countries {
		fmt.Printf("%v - %v\n", code, ISO3166[strings.ToUpper(code)])
	}
	return 0
}

// validateCountry rejects a -country value that is absent from Hola's catalog
// before the expensive bootstrap starts: otherwise zgettunnels would answer
// with an empty ip_list and startup would degenerate into an endless
// "empty response" retry loop. Hola lists lowercase codes; Countries() adds
// "gb" as an alias of "uk", so both spellings pass. A catalog fetch failure
// is only a warning — the check is advisory and bootstrap remains the source
// of truth.
func validateCountry(client *hola.Client, try func(string, func() error) error, logger *applog.CondLogger, country string, timeout time.Duration) error {
	countries, err := fetchCountries(client, try, timeout)
	if err != nil {
		logger.Warning("Unable to validate -country %q against Hola's catalog: %v", country, err)
		return nil
	}
	wanted := strings.ToLower(country)
	for _, code := range countries {
		if code == wanted {
			return nil
		}
	}
	return fmt.Errorf("country %q is not available in Hola. Available countries: %s (use -list-countries to see their names)",
		wanted, strings.Join(countries, ", "))
}

// printProxies fetches a list of agents and prints them as CSV with the
// auth header that the caller would need to use them.
func printProxies(client *hola.Client, try func(string, func() error) error, logger *applog.CondLogger,
	country, proxyType string, limit uint, timeout, backoffInitial, backoffDeadline time.Duration,
) int {
	var (
		tunnels  *hola.ZGetTunnelsResponse
		userUUID string
		err      error
		txRes    bool
		txErr    error
	)
	err = try("list proxies", func() error {
		txRes, txErr = client.EnsureTransaction(context.Background(), timeout, func(ctx context.Context, hc *http.Client) bool {
			userUUID = hola.NewUserUUID()
			tunnels, err = client.Tunnels(ctx, logger, country, proxyType, limit, timeout, backoffInitial, backoffDeadline, hc, userUUID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Transaction error: %v. Retrying with the fallback mechanism...\n", err)
				return false
			}
			return true
		})
		if txErr != nil {
			return fmt.Errorf("transaction recovery mechanism failure: %w", txErr)
		}
		if !txRes {
			return fmt.Errorf("all fallback proxies failed, last error: %w", err)
		}
		return nil
	})
	if err != nil {
		return 3
	}
	wr := csv.NewWriter(os.Stdout)
	login := hola.TemplateLogin(userUUID)
	password := tunnels.AgentKey
	fmt.Println("Login:", login)
	fmt.Println("Password:", password)
	fmt.Println("Proxy-Authorization:", core.BasicAuthHeader(login, password))
	fmt.Println("")
	wr.Write([]string{"host", "ip_address", "direct", "peer", "hola", "trial", "trial_peer", "vendor"})
	for host, ip := range tunnels.IPList {
		if PROTOCOL_WHITELIST[tunnels.Protocol[host]] {
			wr.Write([]string{
				host,
				ip,
				strconv.FormatUint(uint64(tunnels.Port.Direct), 10),
				strconv.FormatUint(uint64(tunnels.Port.Peer), 10),
				strconv.FormatUint(uint64(tunnels.Port.Hola), 10),
				strconv.FormatUint(uint64(tunnels.Port.Trial), 10),
				strconv.FormatUint(uint64(tunnels.Port.TrialPeer), 10),
				tunnels.Vendor[host],
			})
		}
	}
	wr.Flush()
	return 0
}
