package multipass

import "testing"

func TestListNetworks_RealCapture(t *testing.T) {
	runner, _ := fakeRunner(t, map[string]string{
		"networks --format json": loadFixture(t, "networks.json"),
	})
	c := NewClientWithRunner(discardLogger(), runner)
	nets, err := c.ListNetworks()
	if err != nil {
		t.Fatalf("networks: %v", err)
	}
	if len(nets) != 4 {
		t.Fatalf("want 4 networks, got %d", len(nets))
	}
	// Verify the real capture's first entry came through intact.
	if nets[0].Name != "en0" || nets[0].Type != "wifi" || nets[0].Description != "Wi-Fi" {
		t.Errorf("first network: %+v", nets[0])
	}
	// Ethernet entries preserved.
	var ethernetCount int
	for _, n := range nets {
		if n.Type == "ethernet" {
			ethernetCount++
		}
	}
	if ethernetCount != 3 {
		t.Errorf("ethernet count: got %d, want 3", ethernetCount)
	}
}

func TestListNetworks_MalformedJSON(t *testing.T) {
	runner := func(args ...string) (string, error) { return "not json", nil }
	c := NewClientWithRunner(discardLogger(), runner)
	if _, err := c.ListNetworks(); err == nil {
		t.Error("expected parse error")
	}
}
