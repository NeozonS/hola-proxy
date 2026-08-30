package surfclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChromeWindowsUAMatchesImpersonate(t *testing.T) {
	ua := ChromeWindowsUA()
	if !strings.Contains(ua, "Chrome/150.") {
		t.Fatalf("ChromeWindowsUA = %q, want Chrome/150", ua)
	}
	if !strings.Contains(ua, "Windows NT 10.0") {
		t.Fatalf("ChromeWindowsUA = %q, want Windows", ua)
	}
	if got := ChromeProdVersion(); !strings.HasPrefix(got, "150.") {
		t.Fatalf("ChromeProdVersion = %q, want 150.*", got)
	}
}

func TestExtensionOriginHeaders(t *testing.T) {
	var got http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		io.WriteString(w, "ok")
	}))
	t.Cleanup(ts.Close)

	client := New(Options{ExtensionOrigin: HolaExtOrigin, Session: true})
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got.Get("Origin") != HolaExtOrigin {
		t.Errorf("Origin = %q, want %q", got.Get("Origin"), HolaExtOrigin)
	}
	if got.Get("Sec-Fetch-Site") != "none" {
		t.Errorf("Sec-Fetch-Site = %q, want none", got.Get("Sec-Fetch-Site"))
	}
	if got.Get("Sec-Fetch-Mode") != "cors" {
		t.Errorf("Sec-Fetch-Mode = %q, want cors", got.Get("Sec-Fetch-Mode"))
	}
	if got.Get("Sec-Fetch-Dest") != "empty" {
		t.Errorf("Sec-Fetch-Dest = %q, want empty", got.Get("Sec-Fetch-Dest"))
	}
	if got.Get("Accept") != "*/*" {
		t.Errorf("Accept = %q, want */*", got.Get("Accept"))
	}
	if got.Get("Referer") != "" {
		t.Errorf("Referer = %q, want empty", got.Get("Referer"))
	}
	if ua := got.Get("User-Agent"); !strings.Contains(ua, "Chrome/150.") {
		t.Errorf("User-Agent = %q, want Chrome/150", ua)
	}
}
