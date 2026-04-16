package multipass

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// TransferFromVM streams a file from the VM to the provided writer.
func (c *Client) TransferFromVM(vmName, remotePath string, w io.Writer) error {
	if err := ValidateVMName(vmName); err != nil {
		return err
	}
	src := vmName + ":" + remotePath
	cmd := exec.Command("multipass", "transfer", src, "-")
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	c.logger.Debug("exec", "cmd", "multipass transfer "+src+" -")
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		return fmt.Errorf("transfer from VM: %w\n%s", err, errMsg)
	}
	return nil
}

// TransferToVM streams data from the reader to a file in the VM.
func (c *Client) TransferToVM(vmName, remotePath string, r io.Reader) error {
	if err := ValidateVMName(vmName); err != nil {
		return err
	}
	dst := vmName + ":" + remotePath
	cmd := exec.Command("multipass", "transfer", "-", dst)
	cmd.Stdin = r
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	c.logger.Debug("exec", "cmd", "multipass transfer - "+dst)
	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		return fmt.Errorf("transfer to VM: %w\n%s", err, errMsg)
	}
	return nil
}
