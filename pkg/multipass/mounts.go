package multipass

import "fmt"

// ListMounts returns the mounts for a VM (uses the JSON info endpoint).
func (c *Client) ListMounts(vmName string) ([]MountInfo, error) {
	vm, err := c.GetVMInfo(vmName)
	if err != nil {
		return nil, err
	}
	return vm.Mounts, nil
}

// AddMount mounts a host path into a VM.
func (c *Client) AddMount(vmName, source, target string) error {
	_, err := c.run("mount", source, fmt.Sprintf("%s:%s", vmName, target))
	return err
}

// RemoveMount unmounts a path from a VM.
func (c *Client) RemoveMount(vmName, target string) error {
	_, err := c.run("umount", fmt.Sprintf("%s:%s", vmName, target))
	return err
}
