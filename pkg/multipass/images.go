package multipass

import (
	"encoding/json"
	"fmt"
	"sort"
)

// FindImages returns available images and blueprints from multipass find.
func (c *Client) FindImages() ([]ImageInfo, error) {
	output, err := c.run("find", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("find images: %w", err)
	}
	var resp findJSONResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse images: %w", err)
	}

	var results []ImageInfo

	for name, img := range resp.Images {
		results = append(results, ImageInfo{
			Name:    name,
			Aliases: img.Aliases,
			OS:      img.OS,
			Release: img.Release,
			Remote:  img.Remote,
			Version: img.Version,
			Type:    "image",
		})
	}

	for name, img := range resp.Blueprints {
		results = append(results, ImageInfo{
			Name:    name,
			Aliases: img.Aliases,
			OS:      img.OS,
			Release: img.Release,
			Remote:  img.Remote,
			Version: img.Version,
			Type:    "blueprint",
		})
	}

	// Sort: images first, then blueprints, alphabetical within each group
	sort.Slice(results, func(i, j int) bool {
		if results[i].Type != results[j].Type {
			return results[i].Type == "image"
		}
		return results[i].Name < results[j].Name
	})

	return results, nil
}
