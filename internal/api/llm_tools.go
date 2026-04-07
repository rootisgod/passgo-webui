package api

import "encoding/json"

type toolDef struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

var allowedTools map[string]bool

// readOnlyTools are tools safe to expose in read-only mode.
var readOnlyTools = map[string]bool{
	"list_vms":                   true,
	"get_vm_info":                true,
	"list_snapshots":             true,
	"list_networks":              true,
	"list_groups":                true,
	"list_cloud_init_templates":  true,
	"get_cloud_init_template":    true,
	"list_playbooks":             true,
	"get_playbook":               true,
}

// destructiveTools require user confirmation before execution.
var destructiveTools = map[string]bool{
	"delete_vm":                  true,
	"restore_snapshot":           true,
	"delete_snapshot":            true,
	"delete_cloud_init_template": true,
	"delete_playbook":            true,
}

var chatTools []toolDef

func init() {
	chatTools = []toolDef{
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_vms",
				Description: "List all virtual machines with their current state, IPs, and resource usage",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "get_vm_info",
				Description: "Get detailed information about a specific virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the virtual machine"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "start_vm",
				Description: "Start a stopped or suspended virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the VM to start"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "stop_vm",
				Description: "Stop a running virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the VM to stop"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "suspend_vm",
				Description: "Suspend a running virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the VM to suspend"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "delete_vm",
				Description: "Delete a virtual machine. Use purge=true to permanently remove it.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the VM to delete"},
						"purge":{"type":"boolean","description":"If true, permanently remove the VM (cannot be recovered)","default":false}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "recover_vm",
				Description: "Recover a previously deleted virtual machine (only works if not purged)",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the VM to recover"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "create_vm",
				Description: "Create and launch a new virtual machine. All parameters are optional — defaults will be used if omitted. VM names must be unique — check the current VM state before choosing a name to avoid conflicts with existing or in-progress VMs. Use cloud_init to apply a cloud-init template during launch — this is far more reliable than running commands manually via exec_command after creation.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name for the new VM (auto-generated if omitted)"},
						"image":{"type":"string","description":"Ubuntu image/release to use (e.g. '24.04', 'noble'). Defaults to latest LTS."},
						"cpus":{"type":"integer","description":"Number of CPUs","default":1},
						"memory_mb":{"type":"integer","description":"Memory in megabytes","default":1024},
						"disk_gb":{"type":"integer","description":"Disk size in gigabytes","default":5},
						"cloud_init":{"type":"string","description":"Name of a cloud-init template to apply (e.g. 'install-docker.yaml'). Use list_cloud_init_templates to see available templates."}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "exec_command",
				Description: "Execute a command inside a running virtual machine and return its output. IMPORTANT: Shell pipes (|), redirects (>, >>), and chaining (&&, ;) are NOT supported — each call runs a single command. To pipe, use 'bash -c \"cmd1 | cmd2\"' as the command.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM to execute the command in"},
						"command":{"type":"array","items":{"type":"string"},"description":"Command and arguments to execute (e.g. [\"ls\",\"-la\",\"/tmp\"])"}
					},
					"required":["vm","command"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "create_snapshot",
				Description: "Create a snapshot of a virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM to snapshot"},
						"name":{"type":"string","description":"Name for the snapshot"},
						"comment":{"type":"string","description":"Optional description of the snapshot"}
					},
					"required":["vm","name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_snapshots",
				Description: "List all snapshots for a virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM"}
					},
					"required":["vm"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "restore_snapshot",
				Description: "Restore a virtual machine to a previous snapshot. This is destructive — current state will be lost.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM"},
						"snapshot":{"type":"string","description":"Name of the snapshot to restore"}
					},
					"required":["vm","snapshot"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "delete_snapshot",
				Description: "Delete a snapshot from a virtual machine",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM"},
						"snapshot":{"type":"string","description":"Name of the snapshot to delete"}
					},
					"required":["vm","snapshot"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_networks",
				Description: "List available network interfaces that VMs can bridge to",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_groups",
				Description: "List all VM groups and which VMs are assigned to each group",
				Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "create_group",
				Description: "Create a new group for organizing VMs in the sidebar",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name for the new group"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "rename_group",
				Description: "Rename an existing VM group",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"old_name":{"type":"string","description":"Current name of the group"},
						"new_name":{"type":"string","description":"New name for the group"}
					},
					"required":["old_name","new_name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "delete_group",
				Description: "Delete a VM group. VMs in the group are unassigned, not deleted.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name of the group to delete"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "assign_vm_to_group",
				Description: "Move a VM into a group, or remove it from its current group by setting group to empty string",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"vm":{"type":"string","description":"Name of the VM"},
						"group":{"type":"string","description":"Name of the group to assign to, or empty string to unassign"}
					},
					"required":["vm","group"]
				}`),
			},
		},
		// Cloud-Init templates
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_cloud_init_templates",
				Description: "List all available cloud-init templates (both built-in and user-created)",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "get_cloud_init_template",
				Description: "Get the content of a specific cloud-init template by name",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Template filename (e.g. 'docker-setup.yaml')"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "create_cloud_init_template",
				Description: "Create a new cloud-init template file. Content MUST start with '#cloud-config' on the first line and be valid YAML. Filename must end in .yaml or .yml.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Template filename (must end in .yaml or .yml, e.g. 'docker-setup.yaml')"},
						"content":{"type":"string","description":"Cloud-init YAML content (must start with '#cloud-config')"}
					},
					"required":["name","content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "update_cloud_init_template",
				Description: "Update an existing user-created cloud-init template. Cannot modify built-in templates.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Template filename to update"},
						"content":{"type":"string","description":"New cloud-init YAML content (must start with '#cloud-config')"}
					},
					"required":["name","content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "delete_cloud_init_template",
				Description: "Delete a user-created cloud-init template. Cannot delete built-in templates.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Template filename to delete"}
					},
					"required":["name"]
				}`),
			},
		},
		// Ansible playbooks
		{
			Type: "function",
			Function: toolFunction{
				Name:        "list_playbooks",
				Description: "List all Ansible playbooks stored on the server",
				Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "get_playbook",
				Description: "Get the content of a specific Ansible playbook by name",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Playbook filename (e.g. 'setup-web.yml')"}
					},
					"required":["name"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "create_playbook",
				Description: "Create a new Ansible playbook file. Content must be valid Ansible YAML. Filename must end in .yaml or .yml.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Playbook filename (must end in .yaml or .yml)"},
						"content":{"type":"string","description":"Ansible playbook YAML content"}
					},
					"required":["name","content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "update_playbook",
				Description: "Update an existing Ansible playbook.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Playbook filename to update"},
						"content":{"type":"string","description":"New Ansible playbook YAML content"}
					},
					"required":["name","content"]
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "delete_playbook",
				Description: "Delete an Ansible playbook.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Playbook filename to delete"}
					},
					"required":["name"]
				}`),
			},
		},
	}

	allowedTools = make(map[string]bool, len(chatTools))
	for _, t := range chatTools {
		allowedTools[t.Function.Name] = true
	}
}

// filterToolsForMode returns only read-only tools when readOnly is true,
// or the full set when false.
func filterToolsForMode(readOnly bool) []toolDef {
	if !readOnly {
		return chatTools
	}
	var filtered []toolDef
	for _, t := range chatTools {
		if readOnlyTools[t.Function.Name] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
