package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/ssh"
)

type installSSHKeyRequest struct {
	PublicKey string `json:"public_key"`
}

func (s *Server) handleInstallSSHKey(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	if s.guardTemplateMutation(w, r, name, "installing SSH keys on") {
		return
	}

	var req installSSHKeyRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	publicKey := strings.TrimSpace(req.PublicKey)
	if publicKey == "" && s.cfg != nil && s.cfg.VMDefaults != nil {
		publicKey = strings.TrimSpace(s.cfg.VMDefaults.SSHPublicKey)
	}
	if err := validateSSHPublicKey(publicKey); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.mp.ExecInVM(name, []string{
		"sh", "-c", installSSHPublicKeyScript, "sh", publicKey,
	}); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "install_ssh_key", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "install_ssh_key", name, "success", "")
	writeMessage(w, "SSH public key installed")
}

func validateSSHPublicKey(publicKey string) error {
	if publicKey == "" {
		return errBadSSHPublicKey("ssh public key is required")
	}
	if len(publicKey) > 8192 {
		return errBadSSHPublicKey("ssh public key is too long")
	}
	_, _, _, rest, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return errBadSSHPublicKey("ssh public key is invalid")
	}
	if strings.TrimSpace(string(rest)) != "" {
		return errBadSSHPublicKey("only one ssh public key is allowed")
	}
	return nil
}

type errBadSSHPublicKey string

func (e errBadSSHPublicKey) Error() string {
	return string(e)
}

const installSSHPublicKeyScript = `
set -eu
key="$1"
home="${HOME:-/home/ubuntu}"
ssh_dir="$home/.ssh"
auth="$ssh_dir/authorized_keys"
mkdir -p "$ssh_dir"
touch "$auth"
chmod 700 "$ssh_dir"
if ! grep -qxF "$key" "$auth"; then
  printf '%s\n' "$key" >> "$auth"
fi
chmod 600 "$auth"
`
