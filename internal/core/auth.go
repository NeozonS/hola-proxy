package core

import "encoding/base64"

// AuthProvider returns a Proxy-Authorization header value. Returning a closure
// (instead of a static string) lets the value rotate over time.
type AuthProvider func() string

// BasicAuthHeader builds a "basic <base64(login:password)>" header value.
func BasicAuthHeader(login, password string) string {
	return "basic " + base64.StdEncoding.EncodeToString(
		[]byte(login+":"+password))
}
