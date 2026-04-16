package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type inventoryHost struct {
	name string
	ip   string
}

// generateInventoryYAML builds an Ansible inventory YAML string from running VMs.
// filterVMs limits to specific VM names (nil/empty = all running VMs).
func (s *Server) generateInventoryYAML(filterVMs []string, user, sshKeyPath string) (string, error) {
	vms, err := s.mp.ListVMs()
	if err != nil {
		return "", fmt.Errorf("failed to list VMs: %w", err)
	}

	s.cfgMu.Lock()
	groups := make([]string, len(s.cfg.Groups))
	copy(groups, s.cfg.Groups)
	vmGroups := make(map[string]string, len(s.cfg.VMGroups))
	for k, v := range s.cfg.VMGroups {
		vmGroups[k] = v
	}
	s.cfgMu.Unlock()

	// Build filter set
	filterSet := make(map[string]bool, len(filterVMs))
	for _, name := range filterVMs {
		filterSet[name] = true
	}

	var hosts []inventoryHost
	for _, vm := range vms {
		if vm.State != "Running" || len(vm.IPv4) == 0 {
			continue
		}
		if len(filterSet) > 0 && !filterSet[vm.Name] {
			continue
		}
		hosts = append(hosts, inventoryHost{name: vm.Name, ip: vm.IPv4[0]})
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].name < hosts[j].name })

	var b strings.Builder
	b.WriteString("all:\n")

	// vars
	b.WriteString("  vars:\n")
	fmt.Fprintf(&b, "    ansible_user: %s\n", user)
	if sshKeyPath != "" {
		fmt.Fprintf(&b, "    ansible_ssh_private_key_file: %s\n", sshKeyPath)
	}

	// hosts
	b.WriteString("  hosts:\n")
	if len(hosts) == 0 {
		b.WriteString("    {}\n")
	} else {
		for _, h := range hosts {
			fmt.Fprintf(&b, "    %s:\n      ansible_host: %s\n", h.name, h.ip)
		}
	}

	// children (groups)
	groupHosts := make(map[string][]inventoryHost)
	for _, h := range hosts {
		if g, ok := vmGroups[h.name]; ok {
			groupHosts[g] = append(groupHosts[g], h)
		}
	}

	var activeGroups []string
	for _, g := range groups {
		if len(groupHosts[g]) > 0 {
			activeGroups = append(activeGroups, g)
		}
	}

	if len(activeGroups) > 0 {
		b.WriteString("  children:\n")
		for _, g := range activeGroups {
			fmt.Fprintf(&b, "    %s:\n      hosts:\n", g)
			for _, h := range groupHosts[g] {
				fmt.Fprintf(&b, "        %s: {}\n", h.name)
			}
		}
	}

	return b.String(), nil
}

func (s *Server) handleAnsibleInventory(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if user == "" {
		user = "ubuntu"
	}
	sshKey := r.URL.Query().Get("ssh_key")
	if sshKey == "" && s.cfg.VMDefaults != nil {
		sshKey = s.cfg.VMDefaults.SSHPrivateKey
	}
	if sshKey == "" {
		sshKey = multipass.FindMultipassSSHKey()
	}

	var filterVMs []string
	if vm := r.URL.Query().Get("vm"); vm != "" {
		filterVMs = []string{vm}
	}

	inventory, err := s.generateInventoryYAML(filterVMs, user, sshKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="inventory.yml"`)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(inventory))
}
