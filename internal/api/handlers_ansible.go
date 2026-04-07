package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func (s *Server) handleAnsibleInventory(w http.ResponseWriter, r *http.Request) {
	user := r.URL.Query().Get("user")
	if user == "" {
		user = "ubuntu"
	}
	sshKey := r.URL.Query().Get("ssh_key")
	if sshKey == "" && s.cfg.VMDefaults != nil {
		sshKey = s.cfg.VMDefaults.SSHPrivateKey
	}

	vms, err := s.mp.ListVMs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list VMs")
		return
	}

	s.groupMu.Lock()
	groups := make([]string, len(s.cfg.Groups))
	copy(groups, s.cfg.Groups)
	vmGroups := make(map[string]string, len(s.cfg.VMGroups))
	for k, v := range s.cfg.VMGroups {
		vmGroups[k] = v
	}
	s.groupMu.Unlock()

	// Optional: filter to a single VM
	filterVM := r.URL.Query().Get("vm")

	// Collect running VMs with IPs
	type hostEntry struct {
		name string
		ip   string
	}
	var hosts []hostEntry
	for _, vm := range vms {
		if vm.State != "Running" || len(vm.IPv4) == 0 {
			continue
		}
		if filterVM != "" && vm.Name != filterVM {
			continue
		}
		hosts = append(hosts, hostEntry{name: vm.Name, ip: vm.IPv4[0]})
	}
	sort.Slice(hosts, func(i, j int) bool { return hosts[i].name < hosts[j].name })

	var b strings.Builder
	b.WriteString("all:\n")

	// vars
	b.WriteString("  vars:\n")
	fmt.Fprintf(&b, "    ansible_user: %s\n", user)
	if sshKey != "" {
		fmt.Fprintf(&b, "    ansible_ssh_private_key_file: %s\n", sshKey)
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
	groupHosts := make(map[string][]hostEntry)
	for _, h := range hosts {
		if g, ok := vmGroups[h.name]; ok {
			groupHosts[g] = append(groupHosts[g], h)
		}
	}

	// Only emit children if there are non-empty groups
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

	w.Header().Set("Content-Type", "text/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="inventory.yml"`)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(b.String()))
}
