// Package version queries Google APIs to discover the latest stable Chrome
// version and the latest Hola browser-extension version. The values are used
// to build a believable User-Agent and to mimic the extension to Hola's API.
package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/surfclient"
)

type chromeVerResponse struct {
	Versions [1]struct {
		Version string `json:"version"`
	} `json:"versions"`
}

const chromeVerURL = "https://versionhistory.googleapis.com/v1/chrome/platforms/win/channels/stable/versions?alt=json&orderBy=version+desc&pageSize=1&prettyPrint=false"

// GetChromeVer returns the latest stable Chrome version for Windows.
func GetChromeVer(ctx context.Context, dialer core.ContextDialer) (string, error) {
	httpClient := surfclient.New(surfclient.Options{Dialer: dialer})
	defer httpClient.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, "GET", chromeVerURL, nil)
	if err != nil {
		return "", fmt.Errorf("chrome browser version request construction failed: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chrome browser version request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chrome browser version request failed: bad status code: %d", resp.StatusCode)
	}

	var out chromeVerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("unable to decode chrome browser version response: %w", err)
	}
	return out.Versions[0].Version, nil
}
