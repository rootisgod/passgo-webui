package multipass

import (
	"encoding/json"
	"fmt"
)

// ListNetworks returns available network interfaces for bridged networking.
func (c *Client) ListNetworks() ([]NetworkInfo, error) {
	output, err := c.run("networks", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}
	var resp networksJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse networks: %w", err)
	}
	return resp.List, nil
}
