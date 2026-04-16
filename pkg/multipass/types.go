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
	MemoryUsage    string      `json:"memory_usage"`
	MemoryTotal    string      `json:"memory_total"`
	MemoryUsageRaw int64       `json:"memory_usage_raw"`
	MemoryTotalRaw int64       `json:"memory_total_raw"`
	DiskUsageRaw   int64       `json:"disk_usage_raw"`
	DiskTotalRaw   int64       `json:"disk_total_raw"`
	Mounts         []MountInfo `json:"mounts"`
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

// ImageInfo represents an available image or blueprint from multipass find.
type ImageInfo struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
	OS      string   `json:"os"`
	Release string   `json:"release"`
	Remote  string   `json:"remote"`
	Version string   `json:"version"`
	Type    string   `json:"type"` // "image" or "blueprint"
}

// findJSONResponse is the response from multipass find --format json.
type findJSONResponse struct {
	Images     map[string]findJSONImage `json:"images"`
	Blueprints map[string]findJSONImage `json:"blueprints (deprecated)"`
	Errors     []string                 `json:"errors"`
}

type findJSONImage struct {
	Aliases []string `json:"aliases"`
	OS      string   `json:"os"`
	Release string   `json:"release"`
	Remote  string   `json:"remote"`
	Version string   `json:"version"`
}

// --- JSON parsing types for multipass info --format json ---

type infoJSONResponse struct {
	Errors []string                    `json:"errors"`
	Info   map[string]infoJSONVMDetail `json:"info"`
}

type infoJSONVMDetail struct {
	State         string                   `json:"state"`
	ImageHash     string                   `json:"image_hash"`
	ImageRelease  string                   `json:"image_release"`
	Release       string                   `json:"release"`
	CPUs          string                   `json:"cpu_count"`
	Load          []float64                `json:"load"`
	Disks         map[string]infoJSONDisk  `json:"disks"`
	Memory        infoJSONMemory           `json:"memory"`
	Mounts        map[string]infoJSONMount `json:"mounts"`
	IPv4          []string                 `json:"ipv4"`
	// multipass info returns snapshot_count as a string ("4"), not a map.
	// `multipass list --snapshots` is the endpoint that returns full snapshot detail.
	SnapshotCount string                   `json:"snapshot_count"`
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
