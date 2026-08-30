package hola

import (
	"testing"
)

func sampleTunnels() *ZGetTunnelsResponse {
	return &ZGetTunnelsResponse{
		IPList: map[string]string{
			"zagent2.hola.org": "2.2.2.2",
			"zagent1.hola.org": "1.1.1.1",
			"zagent3.hola.org": "3.3.3.3",
		},
		Port: PortMap{
			Direct:    22222,
			Peer:      22223,
			Hola:      22224,
			Trial:     22225,
			TrialPeer: 22226,
		},
	}
}

func TestEndpointsTrialPortForDirectAndLum(t *testing.T) {
	tunnels := sampleTunnels()
	for _, typ := range []string{"direct", "lum", "pool", "virt"} {
		eps, err := Endpoints(tunnels, typ, false, "")
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if len(eps) != 3 {
			t.Fatalf("%s: got %d endpoints, want 3", typ, len(eps))
		}
		if eps[0].TLSName != "zagent1.hola.org" || eps[1].TLSName != "zagent2.hola.org" {
			t.Fatalf("%s: not sorted by name: %s, %s", typ, eps[0].TLSName, eps[1].TLSName)
		}
		for _, ep := range eps {
			if ep.Port != 22225 {
				t.Fatalf("%s: port %d, want trial 22225", typ, ep.Port)
			}
		}
	}
}

func TestEndpointsPeerTrialPort(t *testing.T) {
	eps, err := Endpoints(sampleTunnels(), "peer", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if eps[0].Port != 22226 {
		t.Fatalf("port %d, want trial_peer 22226", eps[0].Port)
	}
}

func TestEndpointsForceNumericPort(t *testing.T) {
	eps, err := Endpoints(sampleTunnels(), "direct", false, "24232")
	if err != nil {
		t.Fatal(err)
	}
	if eps[0].Port != 24232 {
		t.Fatalf("port %d, want 24232", eps[0].Port)
	}
}

func TestEndpointsEmpty(t *testing.T) {
	_, err := Endpoints(&ZGetTunnelsResponse{}, "direct", false, "")
	if err == nil {
		t.Fatal("expected error for empty ip_list")
	}
}

func TestGetEndpointIsFirstSorted(t *testing.T) {
	ep, err := GetEndpoint(sampleTunnels(), "direct", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if ep.TLSName != "zagent1.hola.org" || ep.Host != "1.1.1.1" {
		t.Fatalf("got %+v", ep)
	}
}
