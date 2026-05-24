package version

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/NeozonS/hola-proxy/internal/core"
	"github.com/NeozonS/hola-proxy/internal/surfclient"
)

const defaultProdVersion = "113.0"

// ErrNoVerData is returned when the Chrome Web Store update endpoint returns
// no version data for the requested extension.
var ErrNoVerData = errors.New("no version data returned")

type storeExtUpdateResponse struct {
	XMLName xml.Name `xml:"gupdate"`
	App     *struct {
		AppID       string `xml:"appid,attr"`
		Status      string `xml:"status,attr"`
		UpdateCheck *struct {
			Version string `xml:"version,attr"`
			Status  string `xml:"status,attr"`
		} `xml:"updatecheck"`
	} `xml:"app"`
}

// GetExtVer returns the latest version of a Chrome extension from the
// Chrome Web Store update endpoint. If prodVersion is nil, a sensible default
// is used.
func GetExtVer(ctx context.Context, prodVersion *string, id string, dialer core.ContextDialer) (string, error) {
	if prodVersion == nil {
		v := defaultProdVersion
		prodVersion = &v
	}

	httpClient := surfclient.New(dialer, nil)
	defer httpClient.CloseIdleConnections()

	reqURL := (&url.URL{
		Scheme: "https",
		Host:   "clients2.google.com",
		Path:   "/service/update2/crx",
		RawQuery: url.Values{
			"prodversion":  {*prodVersion},
			"acceptformat": {"crx2,crx3"},
			"x": {url.Values{
				"id": {id},
				"uc": {""},
			}.Encode()},
		}.Encode(),
	}).String()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("chrome web store request construction failed: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chrome web store request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("chrome web store: bad status code: %d", resp.StatusCode)
	}

	var data *storeExtUpdateResponse
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&data); err != nil {
		return "", fmt.Errorf("unmarshaling of chrome web store response failed: %w", err)
	}
	if data != nil && data.App != nil && data.App.UpdateCheck != nil && data.App.UpdateCheck.Version != "" {
		return data.App.UpdateCheck.Version, nil
	}
	return "", ErrNoVerData
}
