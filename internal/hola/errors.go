package hola

import (
	"errors"
	"strings"
)

var (
	// TemporaryBanError is returned by background_init when Hola declines a
	// UUID temporarily.
	TemporaryBanError = errors.New("temporary ban detected")
	// PermanentBanError is returned when the ban won't lift; the caller is
	// expected to give up rather than retry.
	PermanentBanError = errors.New("permanent ban detected")
	// EmptyResponseError is returned by zgettunnels when the agent list is
	// empty, which usually means the country code is wrong or quota exhausted.
	EmptyResponseError = errors.New("empty response")
)

// TemplateLogin renders the basic-auth login string for a UUID. password is
// the AgentKey from ZGetTunnelsResponse.
func TemplateLogin(userUUID string) string {
	var b strings.Builder
	loginTemplate.Execute(&b, map[string]string{
		"uuid": userUUID,
		"prem": "0",
	})
	return b.String()
}
