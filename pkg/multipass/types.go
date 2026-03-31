package multipass

// VMInfo represents full details about a virtual machine.
type VMInfo struct {
	Name        string      `json:"name"`
	State       string      `json:"state"`
	Snapshots   int         `json:"snapshots"`
	IPv4        []string    `json:"ipv4"`
	Release     string      `json:"release"`
	ImageHash   string      `json:"image_hash"`
	CPUs        string      `json:"cpus"`
	Load        string      `json:"load"`
	DiskUsage   string      `json:"disk_usage"`
	DiskTotal   string      `json:"disk_total"`
	MemoryUsage string      `json:"memory_usage"`
	MemoryTotal string      `json:"memory_total"`
	Mounts      []MountInfo `json:"mounts"`
}

// SnapshotInfo represents a snapshot of a VM.
type SnapshotInfo struct {
	Instance string `json:"instance"`
	Name     string `json:"name"`
	Parent   string `json:"parent"`
	Comment  string `json:"comment"`
}

// MountInfo represents a mount point between host and VM.
type MountInfo struct {
	SourcePath string   `json:"source_path"`
	TargetPath string   `json:"target_path"`
	UIDMaps    []string `json:"uid_maps"`
	GIDMaps    []string `json:"gid_maps"`
}

// NetworkInfo represents an available network interface.
type NetworkInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// TemplateOption represents a selectable cloud-init template.
type TemplateOption struct {
	Label   string `json:"label"`
	Path    string `json:"path"`
	BuiltIn bool   `json:"builtIn,omitempty"`
}

// --- JSON parsing types for multipass info --format json ---

type infoJSONResponse struct {
	Errors []string                    `json:"errors"`
	Info   map[string]infoJSONVMDetail `json:"info"`
}

type infoJSONVMDetail struct {
	State       string                       `json:"state"`
	ImageHash   string                       `json:"image_hash"`
	ImageRelease string                      `json:"image_release"`
	Release     string                       `json:"release"`
	CPUs        string                       `json:"cpu_count"`
	Load        []float64                    `json:"load"`
	Disks       map[string]infoJSONDisk      `json:"disks"`
	Memory      infoJSONMemory               `json:"memory"`
	Mounts      map[string]infoJSONMount     `json:"mounts"`
	IPv4        []string                     `json:"ipv4"`
	Snapshots   map[string]infoJSONSnapshot  `json:"snapshots"`
}

type infoJSONDisk struct {
	Used  string `json:"used"`
	Total string `json:"total"`
}

type infoJSONMemory struct {
	Used  int64 `json:"used"`
	Total int64 `json:"total"`
}

type infoJSONMount struct {
	SourcePath  string   `json:"source_path"`
	GIDMappings []string `json:"gid_mappings"`
	UIDMappings []string `json:"uid_mappings"`
}

type infoJSONSnapshot struct {
	Parent  string `json:"parent"`
	Comment string `json:"comment"`
}

// listJSONResponse is the response from multipass list --format json.
type listJSONResponse struct {
	List []listJSONVM `json:"list"`
}

type listJSONVM struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	IPv4    []string `json:"ipv4"`
	Release string   `json:"release"`
}

// networksJSONResponse is the response from multipass networks --format json.
type networksJSONResponse struct {
	List []NetworkInfo `json:"list"`
}
