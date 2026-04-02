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
	"list_vms":       true,
	"get_vm_info":    true,
	"list_snapshots": true,
	"list_networks":  true,
	"list_groups":    true,
}

// destructiveTools require user confirmation before execution.
var destructiveTools = map[string]bool{
	"delete_vm":         true,
	"restore_snapshot":  true,
	"delete_snapshot":   true,
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
				Description: "Create and launch a new virtual machine. All parameters are optional — defaults will be used if omitted.",
				Parameters: json.RawMessage(`{
					"type":"object",
					"properties":{
						"name":{"type":"string","description":"Name for the new VM (auto-generated if omitted)"},
						"image":{"type":"string","description":"Ubuntu image/release to use (e.g. '24.04', 'noble'). Defaults to latest LTS."},
						"cpus":{"type":"integer","description":"Number of CPUs","default":1},
						"memory_mb":{"type":"integer","description":"Memory in megabytes","default":1024},
						"disk_gb":{"type":"integer","description":"Disk size in gigabytes","default":5}
					}
				}`),
			},
		},
		{
			Type: "function",
			Function: toolFunction{
				Name:        "exec_command",
				Description: "Execute a command inside a running virtual machine and return its output",
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
