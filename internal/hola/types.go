package hola

// CountryList is the JSON-decoded list of country codes returned by Hola.
type CountryList []string

// BgInitResponse is the response from /background_init. Blocked indicates
// the user UUID was banned; Permanent distinguishes a ban that won't lift.
type BgInitResponse struct {
	Ver       string `json:"ver"`
	Key       int64  `json:"key"`
	Country   string `json:"country"`
	Blocked   bool   `json:"blocked,omitempty"`
	Permanent bool   `json:"permanent,omitempty"`
}

// PortMap lists the port number used for each tunnel mode an agent supports.
type PortMap struct {
	Direct    uint16 `json:"direct"`
	Hola      uint16 `json:"hola"`
	Peer      uint16 `json:"peer"`
	Trial     uint16 `json:"trial"`
	TrialPeer uint16 `json:"trial_peer"`
}

// ZGetTunnelsResponse is the response from /zgettunnels. IPList maps agent
// hostnames to IPs; Port describes the per-mode port mapping; Protocol
// indicates whether each agent expects HTTP or HTTPS.
type ZGetTunnelsResponse struct {
	AgentKey   string              `json:"agent_key"`
	AgentTypes map[string]string   `json:"agent_types"`
	IPList     map[string]string   `json:"ip_list"`
	Port       PortMap             `json:"port"`
	Protocol   map[string]string   `json:"protocol"`
	Vendor     map[string]string   `json:"vendor"`
	Ztun       map[string][]string `json:"ztun"`
}
